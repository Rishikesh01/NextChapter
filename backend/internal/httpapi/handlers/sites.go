package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/models"
	"github.com/enable-it/nextchapter/backend/internal/sites"
)

// SitesDeps groups the dependencies the sites handlers need. Entries
// is injected for the tracked-hosts read inside [SitesDeps.List];
// keeping the combine in the handler avoids a cross-service dep
// between sites and entries.
type SitesDeps struct {
	Sites   sites.SitesService
	Entries entries.EntriesService
	Logger  *zap.Logger
}

// List implements GET /sites.
//
// @Summary      List the user's site rules plus the hosts they've captured chapters on
// @Description  Returns two parallel lists: `rules` is the per-user site_rule rows (seeded from compiled-in defaults at registration); `tracked_hosts` is the distinct site_host values across the caller's entries. The two are independent — a host can be tracked without a rule, and a rule can exist without any captures.
// @Tags         sites
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Success      200  {object}  models.SiteList
// @Failure      500  {object}  handlers.ErrorBody
// @Router       /sites [get]
func (d SitesDeps) List(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Sites.List"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	rules, err := d.Sites.ListSiteRules(c.Request.Context(), u.ID)
	if err != nil {
		d.Logger.Error("list site rules", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	hosts, err := d.Entries.ListTrackedHosts(c.Request.Context(), u.ID)
	if err != nil {
		d.Logger.Error("list tracked hosts", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusOK, models.SiteList{Rules: rules, TrackedHosts: hosts})
}

// AddRule implements POST /sites/rules.
//
// @Summary      Add a new site rule
// @Description  Adds a per-user site_rule. The regex must compile as a Go regexp and contain named capture groups matching `slug_capture_group` and `chapter_capture_group`. The (user, host) pair is unique — adding a duplicate host returns 422.
// @Tags         sites
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        rule  body      models.SiteRuleNew  true  "rule definition"
// @Success      201   {object}  models.SiteRule
// @Failure      400   {object}  handlers.ErrorBody
// @Failure      422   {object}  handlers.ErrorBody
// @Failure      500   {object}  handlers.ErrorBody
// @Router       /sites/rules [post]
func (d SitesDeps) AddRule(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Sites.AddRule"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	var req models.SiteRuleNew
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
	row, err := d.Sites.AddSiteRule(c.Request.Context(), u.ID, req)
	if err != nil {
		if handled := writeSiteRuleErr(c, err); handled {
			return
		}
		d.Logger.Error("add site rule", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusCreated, row)
}

// EditRule implements PATCH /sites/rules/{id}.
//
// @Summary      Edit a site rule
// @Description  Pointer fields use the absent/present binary: missing fields are left unchanged. The post-patch (host, regex, capture-group) configuration is re-validated; a partial edit that leaves the row in an invalid state is rejected with 422.
// @Tags         sites
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id    path      int                   true  "site rule id"
// @Param        rule  body      models.SiteRulePatch  true  "fields to update"
// @Success      200   {object}  models.SiteRule
// @Failure      400   {object}  handlers.ErrorBody
// @Failure      404   {object}  handlers.ErrorBody
// @Failure      422   {object}  handlers.ErrorBody
// @Failure      500   {object}  handlers.ErrorBody
// @Router       /sites/rules/{id} [patch]
func (d SitesDeps) EditRule(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Sites.EditRule"))
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
	var req models.SiteRulePatch
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
	row, err := d.Sites.EditSiteRule(c.Request.Context(), u.ID, uri.ID, req)
	if err != nil {
		if errors.Is(err, models.ErrSiteRuleNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
				Code:    CodeNotFound,
				Message: "not found",
			}})
			return
		}
		if handled := writeSiteRuleErr(c, err); handled {
			return
		}
		d.Logger.Error("edit site rule", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusOK, row)
}

// RemoveRule implements DELETE /sites/rules/{id}.
//
// @Summary      Remove a site rule
// @Tags         sites
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id   path  int  true  "site rule id"
// @Success      204  "no content"
// @Failure      404  {object}  handlers.ErrorBody
// @Failure      500  {object}  handlers.ErrorBody
// @Router       /sites/rules/{id} [delete]
func (d SitesDeps) RemoveRule(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Sites.RemoveRule"))
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
	if err := d.Sites.RemoveSiteRule(c.Request.Context(), u.ID, uri.ID); err != nil {
		if errors.Is(err, models.ErrSiteRuleNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
				Code:    CodeNotFound,
				Message: "not found",
			}})
			return
		}
		d.Logger.Error("remove site rule", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.Status(http.StatusNoContent)
}

// writeSiteRuleErr maps the content-aware sites-service errors to
// 422 field-level envelopes. Returns true when it handled the error
// (and wrote the response); false when the caller should fall through
// to the generic 500 branch.
//
// Kept out of EditRule's flow for ErrSiteRuleNotFound — that's a 404,
// not a 422, and the EditRule path checks it before calling here.
func writeSiteRuleErr(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, models.ErrSiteRuleInvalidRegex):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
			Code:    CodeValidation,
			Message: "invalid request",
			Fields:  map[string]string{"chapter_url_regex": "must compile as a Go regexp"},
		}})
		return true
	case errors.Is(err, models.ErrSiteRuleMissingCaptureGroup):
		// The typed error carries which JSON field is the source of
		// the failure; recover it via errors.As so the 422 envelope
		// pins the right field.
		var mcg *sites.MissingCaptureGroupError
		field := "slug_capture_group"
		if errors.As(err, &mcg) && mcg.JSONField != "" {
			field = mcg.JSONField
		}
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
			Code:    CodeValidation,
			Message: "invalid request",
			Fields:  map[string]string{field: "named capture group is missing from chapter_url_regex"},
		}})
		return true
	case errors.Is(err, models.ErrSiteRuleHostTaken):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
			Code:    CodeValidation,
			Message: "invalid request",
			Fields:  map[string]string{"host": "you already have a rule for this host"},
		}})
		return true
	}
	return false
}
