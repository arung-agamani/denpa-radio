// Package apierror provides typed errors with HTTP status codes for the Denpa Radio API.
package apierror

// Error is a domain error with a machine-readable code, human-readable message,
// and the HTTP status code that should be sent to the client.
type Error struct {
	Code       string
	Message    string
	HTTPStatus int
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}

// Is reports whether this error matches target. It supports errors.Is so that
// callers can check for specific error kinds without importing this package.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// Sentinel error codes used throughout the API.
const (
	CodeNotFound    = "NOT_FOUND"
	CodeValidation  = "VALIDATION"
	CodeForbidden   = "FORBIDDEN"
	CodeInternal    = "INTERNAL"
	CodeRateLimited = "RATE_LIMITED"
	CodeUnauthorized = "UNAUTHORIZED"
	CodeConflict    = "CONFLICT"
)

// ErrNotFound returns an apierror.Error for missing resources (HTTP 404).
func ErrNotFound(msg string) *Error {
	return &Error{Code: CodeNotFound, Message: msg, HTTPStatus: 404}
}

// ErrValidation returns an apierror.Error for invalid input (HTTP 400).
func ErrValidation(msg string) *Error {
	return &Error{Code: CodeValidation, Message: msg, HTTPStatus: 400}
}

// ErrForbidden returns an apierror.Error for permission denied (HTTP 403).
func ErrForbidden(msg string) *Error {
	return &Error{Code: CodeForbidden, Message: msg, HTTPStatus: 403}
}

// ErrInternal returns an apierror.Error for unexpected server errors (HTTP 500).
func ErrInternal(msg string) *Error {
	return &Error{Code: CodeInternal, Message: msg, HTTPStatus: 500}
}

// ErrRateLimited returns an apierror.Error for rate-limited requests (HTTP 429).
func ErrRateLimited(msg string) *Error {
	return &Error{Code: CodeRateLimited, Message: msg, HTTPStatus: 429}
}

// ErrUnauthorized returns an apierror.Error for unauthenticated requests (HTTP 401).
func ErrUnauthorized(msg string) *Error {
	return &Error{Code: CodeUnauthorized, Message: msg, HTTPStatus: 401}
}

// ErrConflict returns an apierror.Error for resource conflicts (HTTP 409).
func ErrConflict(msg string) *Error {
	return &Error{Code: CodeConflict, Message: msg, HTTPStatus: 409}
}

// Ensure Error satisfies the error interface at compile time.
var _ error = (*Error)(nil)

// Ensure Error is compatible with errors.Is.
var _ interface{ Is(error) bool } = (*Error)(nil)
