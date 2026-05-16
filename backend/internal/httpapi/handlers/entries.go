package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/render"
)

// entriesListQuery binds GET /entries query parameters. SeriesID is a
// pointer so absent and explicit-empty stay distinguishable.
type entriesListQuery struct {
	SeriesID *int64 `form:"series_id" binding:"omitempty,min=1"`
	listPaginationQuery
}

// EntriesDeps groups the dependencies the entries handlers need.
type EntriesDeps struct {
	Entries *entries.Service
	// SeriesCreator is used by /entries/capture when the client supplies
	// new_series_title rather than series_id. Concretely, the series
	// service is wired in here.
	SeriesCreator entries.SeriesCreator
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

func entryToJSON(e entries.Entry) EntryResponse {
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
	u, _ := auth.UserFromContext(c.Request.Context())
	var q entriesListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		render.ValidationError(c, "invalid query", map[string]string{"query": err.Error()})
		return
	}
	items, total, err := d.Entries.List(c.Request.Context(), u.ID, entries.ListParams{
		SeriesID: q.SeriesID, Limit: q.Limit, Offset: q.Offset,
	})
	if err != nil {
		d.Logger.Error("list entries", zap.Error(err))
		render.Internal(c, "")
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
	u, _ := auth.UserFromContext(c.Request.Context())
	var req entries.CaptureParams
	if !bindJSON(c, &req) {
		return
	}
	// ADR-0005 host normalisation: lowercase + strip leading www. before
	// it reaches the service so the (user, host, slug) upsert key matches
	// the canonical form.
	req.SiteHost = strings.TrimPrefix(strings.ToLower(req.SiteHost), "www.")
	res, err := d.Entries.Capture(c.Request.Context(), u.ID, req, d.SeriesCreator)
	if err != nil {
		switch {
		case errors.Is(err, entries.ErrSeriesRequired):
			render.ValidationError(c, "series_id or new_series_title is required when creating a new entry",
				map[string]string{"series_id": "required (or new_series_title)"})
			return
		case errors.Is(err, entries.ErrSeriesNotFound):
			render.ValidationError(c, "series_id does not exist",
				map[string]string{"series_id": "does not exist"})
			return
		}
		d.Logger.Error("capture", zap.Error(err))
		render.Internal(c, "")
		return
	}
	if res.Created {
		c.JSON(http.StatusCreated, entryToJSON(res.Entry))
		return
	}
	c.JSON(http.StatusOK, entryToJSON(res.Entry))
}

// Patch implements PATCH /entries/{id}.
func (d EntriesDeps) Patch(c *gin.Context) {
	u, _ := auth.UserFromContext(c.Request.Context())
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		render.NotFound(c, "")
		return
	}
	var req entries.UpdateParams
	if !bindJSON(c, &req) {
		return
	}
	row, err := d.Entries.Update(c.Request.Context(), u.ID, uri.ID, req)
	if err != nil {
		switch {
		case errors.Is(err, entries.ErrNotFound):
			render.NotFound(c, "")
			return
		case errors.Is(err, entries.ErrSeriesNotFound):
			render.ValidationError(c, "series_id does not exist", map[string]string{"series_id": "does not exist"})
			return
		}
		d.Logger.Error("patch entry", zap.Error(err))
		render.Internal(c, "")
		return
	}
	c.JSON(http.StatusOK, entryToJSON(row))
}

// Delete implements DELETE /entries/{id}.
func (d EntriesDeps) Delete(c *gin.Context) {
	u, _ := auth.UserFromContext(c.Request.Context())
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		render.NotFound(c, "")
		return
	}
	if err := d.Entries.Delete(c.Request.Context(), u.ID, uri.ID); err != nil {
		if errors.Is(err, entries.ErrNotFound) {
			render.NotFound(c, "")
			return
		}
		d.Logger.Error("delete entry", zap.Error(err))
		render.Internal(c, "")
		return
	}
	c.Status(http.StatusNoContent)
}
