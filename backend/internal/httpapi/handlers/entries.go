package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// entriesListQuery binds GET /entries query parameters. SeriesID is
// a pointer so absent and explicit-empty stay distinguishable.
type entriesListQuery struct {
	SeriesID *int64 `form:"series_id" binding:"omitempty,min=1"`
	listPaginationQuery
}

// EntriesDeps groups the dependencies the entries handlers need.
type EntriesDeps struct {
	Entries entries.EntriesService
	// SeriesCreator is used by /entries/capture when the client
	// supplies new_series_title rather than series_id. Concretely,
	// the series service is wired in here.
	SeriesCreator models.SeriesCreator
	Logger        *zap.Logger
}

// EntryResponse is the JSON shape for an entries row. Used by
// GET/POST/PATCH /entries endpoints and embedded in
// SeriesDetailResponse.
type EntryResponse struct {
	ID             int64     `json:"id"`
	SeriesID       int64     `json:"series_id"`
	SiteHost       string    `json:"site_host"`
	SeriesSlug     string    `json:"series_slug"`
	SiteTitle      string    `json:"site_title"`
	LastChapter    float64   `json:"last_chapter"`
	LastURL        string    `json:"last_url"`
	LastCapturedAt time.Time `json:"last_captured_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// EntryListResponse is the GET /entries envelope: paginated entries
// plus total count.
type EntryListResponse struct {
	Items []EntryResponse `json:"items"`
	Total int64           `json:"total"`
}

func entryToJSON(e models.Entry) EntryResponse {
	return EntryResponse{
		ID:             e.ID,
		SeriesID:       e.SeriesID,
		SiteHost:       e.SiteHost,
		SeriesSlug:     e.SeriesSlug,
		SiteTitle:      e.SiteTitle,
		LastChapter:    e.LastChapter,
		LastURL:        e.LastURL,
		LastCapturedAt: e.LastCapturedAt,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
}

// List implements GET /entries.
func (d EntriesDeps) List(c *gin.Context) {
	u, _ := models.UserFromContext(c.Request.Context())
	var q entriesListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
			Code:    CodeValidation,
			Message: "invalid query",
			Fields:  map[string]string{"query": err.Error()},
		}})
		return
	}
	items, total, err := d.Entries.List(c.Request.Context(), u.ID, models.EntryFilter{
		SeriesID: q.SeriesID, Limit: q.Limit, Offset: q.Offset,
	})
	if err != nil {
		d.Logger.Error("list entries", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	out := make([]EntryResponse, 0, len(items))
	for _, e := range items {
		out = append(out, entryToJSON(e))
	}
	c.JSON(http.StatusOK, EntryListResponse{Items: out, Total: total})
}

// Capture implements POST /entries/capture.
func (d EntriesDeps) Capture(c *gin.Context) {
	u, _ := models.UserFromContext(c.Request.Context())
	var req models.EntryCapture
	if err := c.ShouldBindJSON(&req); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    CodeValidation,
				Message: "invalid request",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorBody{Error: ErrorDetail{
			Code:    CodeBadRequest,
			Message: "invalid request body",
		}})
		return
	}
	// ADR-0005 host normalisation: lowercase + strip leading www.
	// before it reaches the service so the (user, host, slug) upsert
	// key matches the canonical form.
	req.SiteHost = strings.TrimPrefix(strings.ToLower(req.SiteHost), "www.")
	entry, created, err := d.Entries.Capture(c.Request.Context(), u.ID, req, d.SeriesCreator)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrEntryCaptureSeriesRequired):
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    CodeValidation,
				Message: "series_id or new_series_title is required when creating a new entry",
				Fields:  map[string]string{"series_id": "required (or new_series_title)"},
			}})
			return
		case errors.Is(err, models.ErrSeriesNotFound):
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    CodeValidation,
				Message: "series_id does not exist",
				Fields:  map[string]string{"series_id": "does not exist"},
			}})
			return
		}
		d.Logger.Error("capture", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	if created {
		c.JSON(http.StatusCreated, entryToJSON(entry))
		return
	}
	c.JSON(http.StatusOK, entryToJSON(entry))
}

// Patch implements PATCH /entries/{id}.
func (d EntriesDeps) Patch(c *gin.Context) {
	u, _ := models.UserFromContext(c.Request.Context())
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
			Code:    CodeNotFound,
			Message: "not found",
		}})
		return
	}
	var req models.EntryPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    CodeValidation,
				Message: "invalid request",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorBody{Error: ErrorDetail{
			Code:    CodeBadRequest,
			Message: "invalid request body",
		}})
		return
	}
	row, err := d.Entries.Patch(c.Request.Context(), u.ID, uri.ID, req)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrEntryNotFound):
			c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
				Code:    CodeNotFound,
				Message: "not found",
			}})
			return
		case errors.Is(err, models.ErrSeriesNotFound):
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    CodeValidation,
				Message: "series_id does not exist",
				Fields:  map[string]string{"series_id": "does not exist"},
			}})
			return
		}
		d.Logger.Error("patch entry", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusOK, entryToJSON(row))
}

// Delete implements DELETE /entries/{id}.
func (d EntriesDeps) Delete(c *gin.Context) {
	u, _ := models.UserFromContext(c.Request.Context())
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
			Code:    CodeNotFound,
			Message: "not found",
		}})
		return
	}
	if err := d.Entries.Delete(c.Request.Context(), u.ID, uri.ID); err != nil {
		if errors.Is(err, models.ErrEntryNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
				Code:    CodeNotFound,
				Message: "not found",
			}})
			return
		}
		d.Logger.Error("delete entry", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.Status(http.StatusNoContent)
}
