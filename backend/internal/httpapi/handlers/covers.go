package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// coverCacheControl is deliberately `private`: a cover is per-user data
// and must never be retained by a shared proxy between two users of the
// same self-hosted instance. Revalidation is cheap because every
// response carries a strong ETag, so a repeat library view costs a 304
// rather than a re-download (ADR-0011 §5).
const coverCacheControl = "private, max-age=0, must-revalidate"

// GetCover implements GET /series/{id}/cover.
//
// Responds with the raw image bytes, not JSON — this is the URL the web
// UI puts in an <img src>. Same-origin means the nc_session cookie
// rides along, so no separate signed-URL scheme is needed.
//
// @Summary      Fetch a series' cover image
// @Description  Returns the raw image bytes (JPEG, PNG or WebP). Honours If-None-Match with a strong ETag.
// @Tags         series
// @Produce      image/jpeg
// @Produce      image/png
// @Produce      image/webp
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id   path      int  true  "series id"
// @Success      200  {file}    binary
// @Success      304  "cover unchanged"
// @Failure      404  {object}  handlers.ErrorBody
// @Failure      500  {object}  handlers.ErrorBody
// @Router       /series/{id}/cover [get]
func (d SeriesDeps) GetCover(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Series.GetCover"))
		writeInternal(c)
		return
	}
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		WriteNotFound(c, "not found")
		return
	}
	cover, err := d.Series.FindSeriesCover(c.Request.Context(), u.ID, uri.ID)
	if err != nil {
		if errors.Is(err, models.ErrCoverNotFound) || errors.Is(err, models.ErrSeriesNotFound) {
			WriteNotFound(c, "not found")
			return
		}
		d.Logger.Error("get series cover", zap.Error(err))
		writeInternal(c)
		return
	}

	etag := `"` + cover.Meta.ETag + `"`
	c.Header("ETag", etag)
	c.Header("Cache-Control", coverCacheControl)
	// A conditional request that already holds these bytes gets a bare
	// 304 — no body, no re-transfer of a 200KB image on every grid render.
	if match := c.GetHeader("If-None-Match"); match != "" && matchesETag(match, etag) {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, cover.Meta.Mime, cover.Bytes)
}

// PutCover implements PUT /series/{id}/cover.
//
// The body is the raw image bytes. The extension fetched them from the
// page the user was on; the backend never dereferences a URL itself
// (ADR-0011 §1). The declared Content-Type is ignored — the service
// sniffs the type from the bytes.
//
// @Summary      Set a series' cover image
// @Description  Body is the raw image bytes (JPEG, PNG or WebP, max 5MiB). The type is sniffed from the bytes; the request Content-Type is not trusted. Optional X-Cover-Source-Url header records provenance.
// @Tags         series
// @Accept       octet-stream
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id   path      int  true  "series id"
// @Success      200  {object}  models.SeriesCoverMeta
// @Failure      404  {object}  handlers.ErrorBody
// @Failure      413  {object}  handlers.ErrorBody
// @Failure      422  {object}  handlers.ErrorBody
// @Failure      500  {object}  handlers.ErrorBody
// @Router       /series/{id}/cover [put]
func (d SeriesDeps) PutCover(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Series.PutCover"))
		writeInternal(c)
		return
	}
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		WriteNotFound(c, "not found")
		return
	}

	// Cap the reader before a single byte is buffered, so an oversized
	// or endless body cannot exhaust memory. MaxBytesReader makes Read
	// fail past the limit rather than silently truncating, which would
	// otherwise store a corrupt image.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, constants.MaxCoverBytes)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, ErrorBody{Error: ErrorDetail{
				Code:    codeValidation,
				Message: "cover image exceeds " + strconv.Itoa(constants.MaxCoverBytes/(1<<20)) + "MiB",
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorBody{Error: ErrorDetail{
			Code:    codeBadRequest,
			Message: "could not read request body",
		}})
		return
	}

	meta, err := d.Series.SetSeriesCover(c.Request.Context(), u.ID, uri.ID, models.CoverUpload{
		Bytes:     raw,
		SourceURL: c.GetHeader("X-Cover-Source-Url"),
	})
	if err != nil {
		switch {
		case errors.Is(err, models.ErrSeriesNotFound):
			WriteNotFound(c, "not found")
		case errors.Is(err, models.ErrCoverEmpty),
			errors.Is(err, models.ErrCoverUnsupportedType),
			errors.Is(err, models.ErrCoverUndecodable):
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    codeValidation,
				Message: err.Error(),
			}})
		default:
			d.Logger.Error("set series cover", zap.Error(err))
			writeInternal(c)
		}
		return
	}
	c.JSON(http.StatusOK, meta)
}

// DeleteCover implements DELETE /series/{id}/cover.
//
// @Summary      Remove a series' cover image
// @Tags         series
// @Produce      json
// @Security     CookieAuth
// @Security     BearerAuth
// @Param        id   path      int  true  "series id"
// @Success      204  "cover removed"
// @Failure      404  {object}  handlers.ErrorBody
// @Failure      500  {object}  handlers.ErrorBody
// @Router       /series/{id}/cover [delete]
func (d SeriesDeps) DeleteCover(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Series.DeleteCover"))
		writeInternal(c)
		return
	}
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		WriteNotFound(c, "not found")
		return
	}
	if err := d.Series.RemoveSeriesCover(c.Request.Context(), u.ID, uri.ID); err != nil {
		if errors.Is(err, models.ErrCoverNotFound) || errors.Is(err, models.ErrSeriesNotFound) {
			WriteNotFound(c, "not found")
			return
		}
		d.Logger.Error("delete series cover", zap.Error(err))
		writeInternal(c)
		return
	}
	c.Status(http.StatusNoContent)
}

// matchesETag reports whether an If-None-Match header value covers the
// supplied strong etag. Handles the wildcard and comma-separated list
// forms from RFC 9110 §13.1.2, and tolerates the weak "W/" prefix a
// proxy may have added on the way through.
func matchesETag(header, etag string) bool {
	if header == "*" {
		return true
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == etag || candidate == "W/"+etag {
			return true
		}
	}
	return false
}

func writeInternal(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
		Code:    codeInternal,
		Message: "internal server error",
	}})
}
