package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is a domain error carrying an HTTP status and a stable code.
type Error struct {
	Status  int
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

// New builds a domain error.
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Wrap attaches an underlying error for logging while keeping a client-safe message.
func (e *Error) Wrap(err error) *Error {
	return &Error{Status: e.Status, Code: e.Code, Message: e.Message, Err: err}
}

// Common constructors.
func BadRequest(msg string) *Error   { return New(http.StatusBadRequest, "bad_request", msg) }
func Unauthorized(msg string) *Error { return New(http.StatusUnauthorized, "unauthorized", msg) }
func Forbidden(msg string) *Error    { return New(http.StatusForbidden, "forbidden", msg) }
func NotFound(msg string) *Error     { return New(http.StatusNotFound, "not_found", msg) }
func Conflict(msg string) *Error     { return New(http.StatusConflict, "conflict", msg) }
func Internal(msg string) *Error     { return New(http.StatusInternalServerError, "internal", msg) }

// As extracts an *Error from an error chain.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
