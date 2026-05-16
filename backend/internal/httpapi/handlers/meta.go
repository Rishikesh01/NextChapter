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
func (d MetaDeps) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.Health{Status: "ok", Version: d.Version})
}

// Root is the placeholder for GET / per ADR-0004. The SPA will replace
// this in a later milestone.
func (d MetaDeps) Root(c *gin.Context) {
	c.String(http.StatusOK, "NextChapter API. See /healthz.\n")
}
