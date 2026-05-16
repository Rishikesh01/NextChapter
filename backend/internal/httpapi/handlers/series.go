package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/models"
	"github.com/enable-it/nextchapter/backend/internal/series"
)

// resourceIDUri is the canonical ":id" path-parameter struct shared
// by every handler that takes a numeric resource id in the URL. gin's
// ShouldBindUri populates the int64 and runs the binding tags; a
// failure or non-positive id maps to 404, matching the previous
// hand-rolled behaviour.
type resourceIDUri struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// SeriesDeps groups the dependencies the series handlers need.
type SeriesDeps struct {
	Series series.SeriesService
	Logger *zap.Logger
}

// List implements GET /series.
//
// @Summary      List the user's tracked series
// @Tags         series
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        status  query     string  false  "filter by status"  Enums(reading,completed,on_hold,dropped,plan_to_read)
// @Param        limit   query     int     false  "page size"         default(50)  minimum(1)  maximum(200)
// @Param        offset  query     int     false  "page offset"       default(0)   minimum(0)
// @Success      200     {object}  models.SeriesList
// @Failure      400     {object}  handlers.ErrorBody
// @Failure      422     {object}  handlers.ErrorBody
// @Failure      500     {object}  handlers.ErrorBody
// @Router       /series [get]
func (d SeriesDeps) List(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Series.List"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	var filter models.SeriesFilter
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
	page, err := d.Series.ListTrackedSeries(c.Request.Context(), u.ID, filter)
	if err != nil {
		d.Logger.Error("list series", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusOK, page)
}

// Create implements POST /series.
//
// @Summary      Track a new series
// @Tags         series
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        series  body      models.SeriesNew  true  "series metadata"
// @Success      201     {object}  models.Series
// @Failure      400     {object}  handlers.ErrorBody
// @Failure      422     {object}  handlers.ErrorBody
// @Failure      500     {object}  handlers.ErrorBody
// @Router       /series [post]
func (d SeriesDeps) Create(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Series.Create"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	var req models.SeriesNew
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
	req.Title = strings.TrimSpace(req.Title)
	row, err := d.Series.TrackSeries(c.Request.Context(), u.ID, req)
	if err != nil {
		d.Logger.Error("create series", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusCreated, row)
}

// Get implements GET /series/{id}.
//
// @Summary      Fetch a single series with its per-site entries
// @Tags         series
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id   path      int  true  "series id"
// @Success      200  {object}  models.SeriesDetail
// @Failure      404  {object}  handlers.ErrorBody
// @Failure      500  {object}  handlers.ErrorBody
// @Router       /series/{id} [get]
func (d SeriesDeps) Get(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Series.Get"))
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
	det, err := d.Series.InspectSeries(c.Request.Context(), u.ID, uri.ID)
	if err != nil {
		if errors.Is(err, models.ErrSeriesNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
				Code:    CodeNotFound,
				Message: "not found",
			}})
			return
		}
		d.Logger.Error("get series", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusOK, det)
}

// Patch implements PATCH /series/{id}.
//
// @Summary      Update mutable fields on a tracked series
// @Description  Pointer fields use the absent/present binary: missing fields are left unchanged. Sending `rating: null` is treated the same as omitting the field — clearing the rating is not supported via PATCH in v1.
// @Tags         series
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id      path      int                 true  "series id"
// @Param        series  body      models.SeriesPatch  true  "fields to update"
// @Success      200     {object}  models.Series
// @Failure      400     {object}  handlers.ErrorBody
// @Failure      404     {object}  handlers.ErrorBody
// @Failure      422     {object}  handlers.ErrorBody
// @Failure      500     {object}  handlers.ErrorBody
// @Router       /series/{id} [patch]
func (d SeriesDeps) Patch(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Series.Patch"))
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
	var req models.SeriesPatch
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
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		req.Title = &t
	}
	row, err := d.Series.EditSeries(c.Request.Context(), u.ID, uri.ID, req)
	if err != nil {
		if errors.Is(err, models.ErrSeriesNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
				Code:    CodeNotFound,
				Message: "not found",
			}})
			return
		}
		d.Logger.Error("patch series", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusOK, row)
}

// Delete implements DELETE /series/{id}.
//
// @Summary      Untrack a series and its entries
// @Tags         series
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id   path  int  true  "series id"
// @Success      204  "no content"
// @Failure      404  {object}  handlers.ErrorBody
// @Failure      500  {object}  handlers.ErrorBody
// @Router       /series/{id} [delete]
func (d SeriesDeps) Delete(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Series.Delete"))
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
	if err := d.Series.UntrackSeries(c.Request.Context(), u.ID, uri.ID); err != nil {
		if errors.Is(err, models.ErrSeriesNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
				Code:    CodeNotFound,
				Message: "not found",
			}})
			return
		}
		d.Logger.Error("delete series", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.Status(http.StatusNoContent)
}
