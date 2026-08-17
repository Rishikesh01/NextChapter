package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error codes used in the [ErrorBody] envelope. The set matches the
// openapi schema description. All codes are package-private; callers
// outside this package use the typed Write* helpers below so the code
// string and HTTP status stay in lock-step.
const (
	codeBadRequest   = "bad_request"
	codeUnauthorized = "unauthorized"
	codeNotFound     = "not_found"
	codeValidation   = "validation"
	codeInternal     = "internal"
)

// ErrorBody is the JSON envelope returned on any non-2xx (except 204).
// Matches the openapi "Error" schema byte-for-byte. Exported because
// the swag annotations on every failure response reference it by name
// (`@Failure 4xx {object} handlers.ErrorBody`); handler code itself
// goes through the Write* helpers below rather than constructing this
// directly.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the inner shape of [ErrorBody]. Exported for the same
// swag-annotation reason as ErrorBody.
type ErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// WriteUnauthorized aborts c with 401 + the canonical "unauthorized"
// envelope. The single way for middleware and handlers to reject a
// missing or invalid credential — keeps the (status, code, message)
// triple in one place instead of every callsite reconstructing it.
func WriteUnauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorBody{Error: ErrorDetail{
		Code:    codeUnauthorized,
		Message: message,
	}})
}

// WriteNotFound aborts c with 404 + the canonical "not_found" envelope.
// Used by the router's NoRoute fallback and by handlers that detect a
// missing resource after auth has succeeded.
func WriteNotFound(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
		Code:    codeNotFound,
		Message: message,
	}})
}
