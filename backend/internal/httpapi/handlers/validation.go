package handlers

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// tagnameRe is the per-tag character class accepted by the `tagname`
// custom validator: lowercase alphanum + dash, first char alphanum,
// length 1..32. Mirrors the spec in [feedback: tags feature]; if this
// regex changes, the corresponding @Description on the series handler
// annotations must be updated in the same commit.
var tagnameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// registerOnce guards the call into the gin validator's RegisterValidation
// against parallel callers. gin holds a single package-level validator
// engine shared across every gin.Engine in the process, so the
// integration test suite (which spins up many engines in parallel) would
// otherwise race on the underlying map. sync.Once ensures the
// registrations happen exactly once per process; we cache the error
// outcome so every caller sees the same result.
var (
	registerOnce sync.Once
	registerErr  error
)

// RegisterCustomValidators wires the project's custom binding-tag
// validators into gin's underlying validator engine. Called once from
// [httpapi.New] at engine setup — never from package init() because
// init() runs per process and is hard to control in test fixtures.
// Safe to call concurrently; only the first invocation does work.
//
// Returns nil on success and an error if the engine is not the
// expected *validator.Validate (a defensive guard for future gin
// upgrades that swap the validator out).
func RegisterCustomValidators() error {
	registerOnce.Do(func() {
		v, ok := binding.Validator.Engine().(*validator.Validate)
		if !ok {
			registerErr = fmt.Errorf("handlers: binding validator engine is not *validator.Validate")
			return
		}
		if err := v.RegisterValidation("tagname", func(fl validator.FieldLevel) bool {
			return tagnameRe.MatchString(fl.Field().String())
		}); err != nil {
			registerErr = fmt.Errorf("handlers: register tagname validator: %w", err)
			return
		}
	})
	return registerErr
}

// validationFieldsFromErr converts a validator.ValidationErrors into a
// map keyed by JSON tag name (lowercased Go field name fallback) with a
// stable human-readable message per validator tag. Used by every
// JSON-body handler that routes a ShouldBindJSON failure, plus any
// handler that does additional cross-field validation on top of the
// bound struct.
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
//
// For `dive` errors against slice elements (e.g. Tags[0] failing
// `tagname`), the indexed suffix is stripped: callers want a stable
// per-field key like "tags", not a per-element "tags[0]". The
// validator's iteration order is non-deterministic across runs and
// the test contract collapses all elements into one field-level
// message.
func jsonFieldNameOf(fe validator.FieldError) string {
	// Both fe.Field() and fe.StructField() include "[0]" / "[1]" /
	// ... for dive errors on slice elements; the project's contract is
	// to surface dive failures under a stable per-field key (e.g.
	// "tags"), not a per-element "tags[0]". Strip any trailing index
	// suffix before consulting the override map / ToLower fallback.
	name := stripIndex(fe.Field())
	if mapped, ok := jsonFieldOverrides[name]; ok {
		return mapped
	}
	return strings.ToLower(name)
}

// stripIndex removes a trailing "[N]" segment from a validator field
// path so dive errors collapse to the parent field name.
func stripIndex(s string) string {
	if i := strings.IndexByte(s, '['); i >= 0 {
		return s[:i]
	}
	return s
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
	"Tags":       "tags",
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
	case "tagname":
		return "must match ^[a-z0-9][a-z0-9-]{0,31}$"
	case "dive":
		return "invalid element"
	default:
		return fmt.Sprintf("failed %s", fe.Tag())
	}
}
