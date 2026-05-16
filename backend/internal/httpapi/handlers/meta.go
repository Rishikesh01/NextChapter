package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/enable-it/nextchapter/backend/internal/models"
)

// MetaDeps groups the meta-endpoint dependencies. Version is surfaced
// from /healthz to make rolling deploys easier to confirm.
type MetaDeps struct {
	Version string
}

// Health implements GET /healthz.
//
// @Summary      Liveness / version probe
// @Description  Returns {"status":"ok"} and the server build version. Unauthenticated.
// @Tags         meta
// @Produce      json
// @Success      200  {object}  models.Health
// @Router       /healthz [get]
func (d MetaDeps) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.Health{Status: "ok", Version: d.Version})
}

// Root is the placeholder for GET / per ADR-0004. The SPA will replace
// this in a later milestone.
//
// @Summary      API root placeholder
// @Description  Placeholder text until the SPA ships (ADR-0004). Unauthenticated.
// @Tags         meta
// @Produce      plain
// @Success      200  {string}  string  "NextChapter API. See /healthz."
// @Router       / [get]
func (d MetaDeps) Root(c *gin.Context) {
	c.String(http.StatusOK, "NextChapter API. See /healthz.\n")
}
