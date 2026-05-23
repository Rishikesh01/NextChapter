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
	// SeriesTracker is used by /entries/capture when the client
	// supplies new_series_title rather than series_id — the series
	// service satisfies the interface natively via TrackImplicitSeries.
	SeriesTracker models.SeriesTracker
	Logger        *zap.Logger
}

// List implements GET /entries.
//
// @Summary      List per-site reading positions
// @Tags         entries
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        series_id  query     int  false  "filter by series id"  minimum(1)
// @Param        limit      query     int  false  "page size"             default(50)  minimum(1)  maximum(200)
// @Param        offset     query     int  false  "page offset"           default(0)   minimum(0)
// @Success      200        {object}  models.EntryList
// @Failure      400        {object}  handlers.ErrorBody
// @Failure      422        {object}  handlers.ErrorBody
// @Failure      500        {object}  handlers.ErrorBody
// @Router       /entries [get]
func (d EntriesDeps) List(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Entries.List"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    codeInternal,
			Message: "internal server error",
		}})
		return
	}
	var filter models.EntryFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    codeValidation,
				Message: "invalid query",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorBody{Error: ErrorDetail{
			Code:    codeBadRequest,
			Message: "invalid query",
		}})
		return
	}
	page, err := d.Entries.ListReadingPositions(c.Request.Context(), u.ID, filter)
	if err != nil {
		d.Logger.Error("list entries", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    codeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusOK, page)
}

// Capture implements POST /entries/capture.
//
// @Summary      Capture or advance a reading position
// @Description  Upserts on (user, site_host, series_slug). The handler lowercases site_host and strips a leading `www.` before persisting. If no entry exists for that key, the caller must supply either `series_id` (existing) or `new_series_title` (the handler will materialise a series row); otherwise 422. Returns 201 on first capture for the (host,slug) key, 200 when an existing entry was advanced.
// @Tags         entries
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        capture  body      models.EntryCapture  true  "captured reading position"
// @Success      200      {object}  models.Entry  "advanced an existing entry"
// @Success      201      {object}  models.Entry  "created a new entry"
// @Failure      400      {object}  handlers.ErrorBody
// @Failure      422      {object}  handlers.ErrorBody
// @Failure      500      {object}  handlers.ErrorBody
// @Router       /entries/capture [post]
func (d EntriesDeps) Capture(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Entries.Capture"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    codeInternal,
			Message: "internal server error",
		}})
		return
	}
	var req models.EntryCapture
	if err := c.ShouldBindJSON(&req); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    codeValidation,
				Message: "invalid request",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorBody{Error: ErrorDetail{
			Code:    codeBadRequest,
			Message: "invalid request body",
		}})
		return
	}
	// Host normalisation: lowercase + strip leading www. before it
	// reaches the service so the (user, host, slug) upsert key
	// matches the canonical form.
	req.SiteHost = strings.TrimPrefix(strings.ToLower(req.SiteHost), "www.")
	entry, created, err := d.Entries.CaptureChapter(c.Request.Context(), u.ID, req, d.SeriesTracker)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrEntryCaptureSeriesRequired):
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    codeValidation,
				Message: "series_id or new_series_title is required when creating a new entry",
				Fields:  map[string]string{"series_id": "required (or new_series_title)"},
			}})
			return
		case errors.Is(err, models.ErrSeriesNotFound):
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    codeValidation,
				Message: "series_id does not exist",
				Fields:  map[string]string{"series_id": "does not exist"},
			}})
			return
		}
		d.Logger.Error("capture", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    codeInternal,
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
//
// @Summary      Adjust an existing entry
// @Description  Pointer fields use the absent/present binary: missing fields are left unchanged.
// @Tags         entries
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id     path      int                true  "entry id"
// @Param        entry  body      models.EntryPatch  true  "fields to update"
// @Success      200    {object}  models.Entry
// @Failure      400    {object}  handlers.ErrorBody
// @Failure      404    {object}  handlers.ErrorBody
// @Failure      422    {object}  handlers.ErrorBody
// @Failure      500    {object}  handlers.ErrorBody
// @Router       /entries/{id} [patch]
func (d EntriesDeps) Patch(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Entries.Patch"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    codeInternal,
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
				Code:    codeValidation,
				Message: "invalid request",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorBody{Error: ErrorDetail{
			Code:    codeBadRequest,
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
				Code:    codeValidation,
				Message: "series_id does not exist",
				Fields:  map[string]string{"series_id": "does not exist"},
			}})
			return
		}
		d.Logger.Error("patch entry", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    codeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusOK, row)
}

// Delete implements DELETE /entries/{id}.
//
// @Summary      Forget a single per-site reading position
// @Tags         entries
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id   path  int  true  "entry id"
// @Success      204  "no content"
// @Failure      404  {object}  handlers.ErrorBody
// @Failure      500  {object}  handlers.ErrorBody
// @Router       /entries/{id} [delete]
func (d EntriesDeps) Delete(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Entries.Delete"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    codeInternal,
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
			Code:    codeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.Status(http.StatusNoContent)
}
