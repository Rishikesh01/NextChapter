package handlers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/enable-it/nextchapter/backend/internal/httpapi/render"
)

// bindJSON binds the request body into out. On parse failure (malformed
// JSON) it writes a 400 BadRequest envelope and returns false. On
// field-level validation failure (validator.ValidationErrors) it writes
// a 422 ValidationError envelope keyed by JSON field name and returns
// false. On success returns true.
//
// All JSON-body handlers should use bindJSON instead of calling
// c.ShouldBindJSON directly.
func bindJSON(c *gin.Context, out any) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			render.ValidationError(c, "invalid request", validationFieldsFromErr(verr))
			return false
		}
		render.BadRequest(c, "invalid request body")
		return false
	}
	return true
}

// validationFieldsFromErr converts a validator.ValidationErrors into a
// map keyed by JSON tag name (lowercased Go field name fallback) with a
// stable human-readable message per validator tag. Used by both bindJSON
// and any handler that does additional cross-field validation on top of
// the bound struct.
func validationFieldsFromErr(verr validator.ValidationErrors) map[string]string {
	out := make(map[string]string, len(verr))
	for _, fe := range verr {
		key := jsonFieldNameOf(fe)
		out[key] = messageFor(fe)
	}
	return out
}

// jsonFieldNameOf returns the wire-level field name for an erroring
// validator.FieldError. Falls back to lowercased Go field name if the
// JSON tag is absent or "-".
func jsonFieldNameOf(fe validator.FieldError) string {
	// fe.Field() gives the Go field name. We don't have direct access
	// to the json tag from a validator.FieldError, so we lowercase as
	// a passable approximation: the wire schema in this project uses
	// snake_case via json tags. The shared helper below maps the known
	// Go field names to their JSON tags; falls back to ToLower.
	if mapped, ok := jsonFieldOverrides[fe.Field()]; ok {
		return mapped
	}
	return strings.ToLower(fe.Field())
}

// jsonFieldOverrides maps Go field names to their JSON tag names where
// the lowercase fallback isn't right (e.g. multi-word fields like
// SiteHost -> site_host). Populated lazily as new request types appear.
var jsonFieldOverrides = map[string]string{
	"SiteHost":   "site_host",
	"SeriesSlug": "series_slug",
	"SiteTitle":  "site_title",
	"SeriesID":   "series_id",
	"URL":        "url",
	"Username":   "username",
	"Password":   "password",
	"Title":      "title",
	"Status":     "status",
	"Rating":     "rating",
	"Notes":      "notes",
	"Chapter":    "chapter",
	"Label":      "label",
}

// messageFor renders a stable error message for a validator tag.
func messageFor(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "required"
	case "min":
		if fe.Kind().String() == "string" {
			return fmt.Sprintf("must be at least %s characters", fe.Param())
		}
		return fmt.Sprintf("must be >= %s", fe.Param())
	case "max":
		if fe.Kind().String() == "string" {
			return fmt.Sprintf("must be at most %s characters", fe.Param())
		}
		return fmt.Sprintf("must be <= %s", fe.Param())
	case "len":
		return fmt.Sprintf("must be exactly %s characters", fe.Param())
	case "gte":
		return fmt.Sprintf("must be >= %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be <= %s", fe.Param())
	case "url":
		return "must be a valid URL"
	case "oneof":
		return "must be one of: " + strings.ReplaceAll(fe.Param(), " ", ", ")
	default:
		return fmt.Sprintf("failed %s", fe.Tag())
	}
}
