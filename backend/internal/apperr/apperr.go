// Package apperr berisi error domain yang diterjemahkan handler menjadi respons HTTP.
package apperr

import (
	"errors"
	"net/http"
)

type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	// RetryAfter (detik) diisi untuk 429.
	RetryAfter int `json:"-"`
}

func (e *Error) Error() string { return e.Message }

func New(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

func BadRequest(msg string) *Error   { return New(http.StatusBadRequest, "bad_request", msg) }
func Unauthorized(msg string) *Error { return New(http.StatusUnauthorized, "unauthorized", msg) }
func Forbidden(msg string) *Error    { return New(http.StatusForbidden, "forbidden", msg) }
func NotFound(msg string) *Error     { return New(http.StatusNotFound, "not_found", msg) }
func Conflict(msg string) *Error     { return New(http.StatusConflict, "conflict", msg) }

// Locked dipakai untuk periode tertutup.
func Locked(msg string) *Error { return New(http.StatusLocked, "period_locked", msg) }

func Validation(msg string, fields map[string]string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: "validation_error", Message: msg, Fields: fields}
}

func TooMany(msg string, retryAfter int) *Error {
	return &Error{Status: http.StatusTooManyRequests, Code: "too_many_requests", Message: msg, RetryAfter: retryAfter}
}

// As mengembalikan *Error bila err (atau pembungkusnya) adalah apperr.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
