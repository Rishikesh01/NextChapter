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
