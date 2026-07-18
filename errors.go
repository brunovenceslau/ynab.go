// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"pkg.venceslau.dev/ynab/internal/transport"
)

// Error is a YNAB API error response. ID carries the API's sub-coded string
// discriminator ("403.1", "404.2", …) — the wire's source of truth; match
// with errors.Is against the sentinel variables, never by parsing ID.
type Error struct {
	StatusCode int
	ID         string
	Name       string
	Detail     string

	// RetryAfter is the server-requested wait before retrying (429 only).
	// Zero means unknown — never "retry immediately".
	RetryAfter time.Duration
}

// Error returns the message "ynab: HTTP <status> <id> <name>: <detail>",
// omitting whatever the response did not carry. It is nil-safe.
func (e *Error) Error() string {
	if e == nil {
		return "ynab: <nil>"
	}
	var b strings.Builder
	b.WriteString("ynab: HTTP ")
	b.WriteString(strconv.Itoa(e.StatusCode))
	if e.ID != "" {
		b.WriteString(" ")
		b.WriteString(e.ID)
	}
	if e.Name != "" {
		b.WriteString(" ")
		b.WriteString(e.Name)
	}
	if e.Detail != "" {
		b.WriteString(": ")
		b.WriteString(e.Detail)
	}
	return b.String()
}

// Is reports whether target is one of the sentinels this error satisfies:
// its exact sub-code AND its status class, so a 403.1 response matches both
// ErrSubscriptionLapsed and ErrForbidden. Unknown sub-codes still match
// their class. It is nil-safe on both sides.
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	if s, ok := sentinelByID[e.ID]; ok && target == s {
		return true
	}
	s, ok := sentinelByStatus[e.StatusCode]
	return ok && target == s
}

// ArgumentError reports a client-side pre-flight validation failure: the
// request was never sent. Only spec-stated invariants are validated (length
// bounds, zero Month, split-sum equality, scheduled-date window) — never
// guessed business rules. Field is empty for cross-field violations.
// Construction failures report the constructor in Op and the failing
// option in Field. Op and Reason are for humans and logs — do not parse
// them.
type ArgumentError struct {
	Op     string
	Field  string
	Reason string
}

// Error returns "ynab: <op>: <field>: <reason>" (field omitted when empty).
// It is nil-safe.
func (e *ArgumentError) Error() string {
	if e == nil {
		return "ynab: <nil>"
	}
	if e.Field == "" {
		return "ynab: " + e.Op + ": " + e.Reason
	}
	return "ynab: " + e.Op + ": " + e.Field + ": " + e.Reason
}

// Sentinel errors for the full YNAB error taxonomy. Class sentinels
// (ErrForbidden, ErrNotFound) match every sub-code of their status;
// specific sentinels match exactly one.
var (
	ErrBadRequest    = errors.New("ynab: bad request (400)")
	ErrNotAuthorized = errors.New("ynab: not authorized (401)")

	ErrForbidden          = errors.New("ynab: forbidden (403)")
	ErrSubscriptionLapsed = errors.New("ynab: subscription lapsed (403.1)")
	ErrTrialExpired       = errors.New("ynab: trial expired (403.2)")
	ErrUnauthorizedScope  = errors.New("ynab: unauthorized scope (403.3)")
	ErrDataLimitReached   = errors.New("ynab: data limit reached (403.4)")

	ErrNotFound         = errors.New("ynab: not found (404)")
	ErrRouteNotFound    = errors.New("ynab: route not found (404.1)")
	ErrResourceNotFound = errors.New("ynab: resource not found (404.2)")

	ErrConflict           = errors.New("ynab: conflict (409)")
	ErrRateLimited        = errors.New("ynab: rate limited (429)")
	ErrServerError        = errors.New("ynab: internal server error (500)")
	ErrServiceUnavailable = errors.New("ynab: service unavailable (503)")
)

// sentinelByID maps the exact wire error.id to its specific sentinel.
var sentinelByID = map[string]error{
	"400":   ErrBadRequest,
	"401":   ErrNotAuthorized,
	"403.1": ErrSubscriptionLapsed,
	"403.2": ErrTrialExpired,
	"403.3": ErrUnauthorizedScope,
	"403.4": ErrDataLimitReached,
	"404.1": ErrRouteNotFound,
	"404.2": ErrResourceNotFound,
	"409":   ErrConflict,
	"429":   ErrRateLimited,
	"500":   ErrServerError,
	"503":   ErrServiceUnavailable,
}

// sentinelByStatus maps the HTTP status to its class sentinel, so unknown
// sub-codes keep matching their class.
var sentinelByStatus = map[int]error{
	400: ErrBadRequest,
	401: ErrNotAuthorized,
	403: ErrForbidden,
	404: ErrNotFound,
	409: ErrConflict,
	429: ErrRateLimited,
	500: ErrServerError,
	503: ErrServiceUnavailable,
}

// maxLenError reports one spec-declared maxLength violation.
func maxLenError(op, field string, limit int) *ArgumentError {
	return &ArgumentError{Op: op, Field: field, Reason: fmt.Sprintf("must be at most %d characters", limit)}
}

// checkRuneMax validates a spec-declared maxLength bound in characters
// (code points), the unit JSON Schema's maxLength counts.
func checkRuneMax(op, field, v string, limit int) error {
	if utf8.RuneCountInString(v) > limit {
		return maxLenError(op, field, limit)
	}
	return nil
}

// checkOptRuneMax is checkRuneMax over a set Optional[string].
func checkOptRuneMax(op, field string, o Optional[string], limit int) error {
	if v, ok := o.Get(); ok {
		return checkRuneMax(op, field, v, limit)
	}
	return nil
}

// decodeWireError maps a non-2xx response to *Error — the decoder the root
// package injects into the transport core. An undecodable or non-envelope
// body still yields *Error carrying the status code; Retry-After is parsed
// best-effort on 429 and never required.
func decodeWireError(status int, body []byte, hdr http.Header) error {
	e := &Error{StatusCode: status}

	var env struct {
		Error *struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Detail string `json:"detail"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Error != nil {
		e.ID, e.Name, e.Detail = env.Error.ID, env.Error.Name, env.Error.Detail
	}
	if status == http.StatusTooManyRequests {
		e.RetryAfter = transport.RetryAfterDelay(hdr.Get("Retry-After"))
	}
	return e
}

// IsRetryable reports whether retrying the operation may succeed, per the
// API's error semantics: true for 429 (wait first — see Error.RetryAfter),
// 500, 503, and transport timeouts/connection failures; false for every
// other 4xx, for *ArgumentError (the request was never sent), and for
// context cancellation. A bare context.DeadlineExceeded is the caller's own
// deadline and is not retryable; a transport timeout — a wrapped net.Error
// whose Timeout() is true — is. It understands wrapped errors.
//
// The built-in retry pipeline already applies these semantics; IsRetryable
// exists for callers orchestrating their own loops (e.g. with
// WithRetryDisabled, or batch jobs deciding requeue vs drop).
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	var argErr *ArgumentError
	if errors.As(err, &argErr) {
		return false
	}

	if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrServerError) || errors.Is(err, ErrServiceUnavailable) {
		return true
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return false // every other API status is a terminal 4xx-class answer
	}

	// Cancellation is the caller's decision — checked before the net.Error
	// walk because in-flight cancellations arrive wrapped in one.
	if errors.Is(err, context.Canceled) {
		return false
	}

	// context.DeadlineExceeded itself implements net.Error, so finding a
	// net.Error is not yet proof of a transport failure: a bare caller
	// deadline must stay non-retryable. The transport case always wraps
	// the deadline (url.Error, OpError, …); the bare case is the naked
	// context value itself — a wrapper-less net.Error that is the deadline.
	var netErr net.Error
	if errors.As(err, &netErr) {
		bareCallerDeadline := errors.Unwrap(netErr) == nil && errors.Is(netErr, context.DeadlineExceeded)
		return !bareCallerDeadline
	}
	return false
}
