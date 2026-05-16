package handlers

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
