package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// EntriesDeps groups the dependencies the entries handlers need.
type EntriesDeps struct {
	Entries entries.EntriesService
	// SeriesCreator is used by /entries/capture when the client
	// supplies new_series_title rather than series_id. Concretely,
	// the series service is wired in here.
	SeriesCreator models.SeriesCreator
	Logger        *zap.Logger
}

// List implements GET /entries.
func (d EntriesDeps) List(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Entries.List"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	var filter models.EntryFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    CodeValidation,
				Message: "invalid query",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorBody{Error: ErrorDetail{
			Code:    CodeBadRequest,
			Message: "invalid query",
		}})
		return
	}
	page, err := d.Entries.ListReadingPositions(c.Request.Context(), u.ID, filter)
	if err != nil {
		d.Logger.Error("list entries", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusOK, page)
}

// Capture implements POST /entries/capture.
func (d EntriesDeps) Capture(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Entries.Capture"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
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
	entry, created, err := d.Entries.CaptureChapter(c.Request.Context(), u.ID, req, d.SeriesCreator)
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
		c.JSON(http.StatusCreated, entry)
		return
	}
	c.JSON(http.StatusOK, entry)
}

// Patch implements PATCH /entries/{id}.
func (d EntriesDeps) Patch(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Entries.Patch"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
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
	row, err := d.Entries.AdjustReadingPosition(c.Request.Context(), u.ID, uri.ID, req)
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
	c.JSON(http.StatusOK, row)
}

// Delete implements DELETE /entries/{id}.
func (d EntriesDeps) Delete(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Entries.Delete"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
			Code:    CodeNotFound,
			Message: "not found",
		}})
		return
	}
	if err := d.Entries.ForgetReadingPosition(c.Request.Context(), u.ID, uri.ID); err != nil {
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
