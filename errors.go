package kannon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Problem codes worth branching on. The server documents the full set; treat
// any other code by its HTTP status.
const (
	CodeUnauthenticated       = "unauthenticated"
	CodeForbidden             = "forbidden"
	CodeNotFound              = "not_found"
	CodeValidationFailed      = "validation_failed"
	CodeNoRecipientsAccepted  = "no_recipients_accepted"
	CodeTooManyRecipients     = "too_many_recipients"
	CodeIdempotencyConflict   = "idempotency_conflict"
	CodeIdempotencyInProgress = "idempotency_in_progress"
	CodeRateLimited           = "rate_limited"
	CodeServiceUnavailable    = "service_unavailable"
)

// APIError is a request the Kannon API refused, decoded from its RFC 9457
// Problem Details.
type APIError struct {
	// StatusCode is the HTTP status of the response.
	StatusCode int
	// Code is the stable, machine-readable problem code, such as
	// CodeNoRecipientsAccepted. It is empty when the response carried no
	// Problem Details.
	Code string
	// Title is the human-readable summary of the problem.
	Title string
	// Detail is the human-readable explanation, when the server gives one.
	Detail string
	// RequestID identifies the request in the server logs.
	RequestID string
	// Errors lists the offending parts of the request. For
	// CodeNoRecipientsAccepted it holds one entry per refused recipient, with
	// a Pointer such as "/recipients/2/email" and the RejectionReason as Code.
	Errors []FieldError
	// RetryAfter is how long the server asks to wait before retrying, zero
	// when it gave no hint.
	RetryAfter time.Duration
}

// FieldError is one offending part of a refused request.
type FieldError struct {
	Code    string `json:"code"`
	Pointer string `json:"pointer"`
	Detail  string `json:"detail"`
}

func (e *APIError) Error() string {
	code := e.Code
	if code == "" {
		code = http.StatusText(e.StatusCode)
	}
	if e.Detail != "" {
		return fmt.Sprintf("kannon: %d %s: %s", e.StatusCode, code, e.Detail)
	}
	return fmt.Sprintf("kannon: %d %s", e.StatusCode, code)
}

type wireProblem struct {
	Title     string       `json:"title"`
	Detail    string       `json:"detail"`
	Code      string       `json:"code"`
	RequestID string       `json:"request_id"`
	Errors    []FieldError `json:"errors"`
}

func newAPIError(res *http.Response, body []byte) *APIError {
	var problem wireProblem
	_ = json.Unmarshal(body, &problem)

	apiErr := &APIError{
		StatusCode: res.StatusCode,
		Code:       problem.Code,
		Title:      problem.Title,
		Detail:     problem.Detail,
		RequestID:  problem.RequestID,
		Errors:     problem.Errors,
	}
	if apiErr.RequestID == "" {
		apiErr.RequestID = res.Header.Get("X-Request-ID")
	}
	if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds > 0 {
		apiErr.RetryAfter = time.Duration(seconds) * time.Second
	}
	return apiErr
}
