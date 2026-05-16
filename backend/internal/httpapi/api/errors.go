// Package api holds the JSON wire types shared between the handlers
// package and middleware that emits the same error envelope. It exists
// solely to break the import cycle that arises when middleware in
// internal/auth needs to write the openapi "Error" shape — and the
// handlers package, which also writes it, already imports
// internal/auth. Keeping these types here lets both sides import a
// neutral package without a render-helper layer.
package api

// Error codes used in the ErrorBody envelope. The set matches the
// openapi schema description.
const (
	CodeBadRequest   = "bad_request"
	CodeUnauthorized = "unauthorized"
	CodeNotFound     = "not_found"
	CodeValidation   = "validation"
	CodeConflict     = "conflict"
	CodeInternal     = "internal"
)

// ErrorBody is the JSON envelope returned on any non-2xx (except 204).
// Matches the openapi "Error" schema byte-for-byte. Every error path
// constructs this directly and calls c.AbortWithStatusJSON.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the inner shape of ErrorBody.
type ErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}
