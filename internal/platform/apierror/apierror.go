// Package apierror is the one error model the API speaks. A handler never
// writes a driver error to a client: it maps to a Code here, and the HTTP
// layer renders exactly one JSON shape (04-api-specification.md §2).
package apierror

import (
	"errors"
	"fmt"
	"net/http"
)

type Code string

const (
	CodeValidation      Code = "VALIDATION"
	CodeUnauthenticated Code = "UNAUTHENTICATED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeTOTPRequired    Code = "TOTP_REQUIRED"
	CodeLeadTime        Code = "LEAD_TIME_VIOLATION"
	CodeOverlap         Code = "OVERLAP_UNACKNOWLEDGED"
	CodeCrossBrand      Code = "CROSS_BRAND_MEMBER"
	CodeSelfApproval    Code = "SELF_APPROVAL"
	CodeReasonRequired  Code = "REASON_REQUIRED"
	CodeNotYourStep     Code = "NOT_YOUR_STEP"
	CodeAlreadyDecided  Code = "ALREADY_DECIDED"
	CodePlanLocked      Code = "PLAN_LOCKED"
	CodeRateLimited     Code = "RATE_LIMITED"
	CodeInternal        Code = "INTERNAL"
)

var status = map[Code]int{
	CodeValidation:      http.StatusUnprocessableEntity,
	CodeUnauthenticated: http.StatusUnauthorized,
	CodeForbidden:       http.StatusForbidden,
	CodeNotFound:        http.StatusNotFound,
	CodeConflict:        http.StatusConflict,
	CodeTOTPRequired:    http.StatusUnauthorized,
	CodeLeadTime:        http.StatusUnprocessableEntity,
	CodeOverlap:         http.StatusConflict,
	CodeCrossBrand:      http.StatusUnprocessableEntity,
	CodeSelfApproval:    http.StatusConflict,
	CodeReasonRequired:  http.StatusBadRequest,
	CodeNotYourStep:     http.StatusForbidden,
	CodeAlreadyDecided:  http.StatusConflict,
	CodePlanLocked:      http.StatusConflict,
	CodeRateLimited:     http.StatusTooManyRequests,
	CodeInternal:        http.StatusInternalServerError,
}

// Error is the only error type that reaches a client.
type Error struct {
	Code    Code              `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	// Cause never leaves the process. It is logged, not rendered.
	Cause error `json:"-"`
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

// HTTPStatus maps the code. An unknown code is a 500 rather than a 200 — a
// missing map entry must never read as success.
func (e *Error) HTTPStatus() int {
	if s, ok := status[e.Code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

func New(c Code, msg string) *Error { return &Error{Code: c, Message: msg} }

func Newf(c Code, format string, a ...any) *Error {
	return &Error{Code: c, Message: fmt.Sprintf(format, a...)}
}

// Wrap keeps the driver error for the log and hides it from the client.
func Wrap(c Code, msg string, cause error) *Error {
	return &Error{Code: c, Message: msg, Cause: cause}
}

func Validation(msg string, fields map[string]string) *Error {
	return &Error{Code: CodeValidation, Message: msg, Fields: fields}
}

func NotFound(what string) *Error {
	return &Error{Code: CodeNotFound, Message: what + " tidak ditemukan"}
}

// As extracts an *Error, or synthesises an INTERNAL one. Every non-nil error
// reaching the HTTP layer becomes an *Error exactly once.
func As(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: CodeInternal, Message: "kesalahan internal", Cause: err}
}
