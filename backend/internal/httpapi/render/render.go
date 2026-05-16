// Package render owns the JSON response envelopes for the HTTP API.
// The Error envelope here matches the openapi schema "Error" byte-for-byte.
package render

import (
	"github.com/gin-gonic/gin"
)

// Error codes used in the Error envelope. Mirror the openapi description.
const (
	CodeUnauthorized = "unauthorized"
	CodeNotFound     = "not_found"
	CodeValidation   = "validation"
	CodeConflict     = "conflict"
	CodeInternal     = "internal"
	CodeBadRequest   = "bad_request"
)

// ErrorBody is the JSON payload returned on any non-2xx (except 204).
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the inner shape of [ErrorBody].
type ErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Error writes a JSON error envelope with the given status code.
func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorBody{Error: ErrorDetail{Code: code, Message: message}})
}

// ValidationError writes a 422 with per-field messages.
func ValidationError(c *gin.Context, message string, fields map[string]string) {
	c.AbortWithStatusJSON(422, ErrorBody{Error: ErrorDetail{
		Code:    CodeValidation,
		Message: message,
		Fields:  fields,
	}})
}

// Unauthorized writes a 401 with the unified message.
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "missing or invalid credentials"
	}
	Error(c, 401, CodeUnauthorized, message)
}

// NotFound writes a 404 with the unified message.
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = "not found"
	}
	Error(c, 404, CodeNotFound, message)
}

// Internal writes a 500 with a generic message; the real error is logged
// upstream.
func Internal(c *gin.Context, message string) {
	if message == "" {
		message = "internal server error"
	}
	Error(c, 500, CodeInternal, message)
}

// BadRequest writes a 400 for malformed JSON or invalid query params.
func BadRequest(c *gin.Context, message string) {
	if message == "" {
		message = "bad request"
	}
	Error(c, 400, CodeBadRequest, message)
}
