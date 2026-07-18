package ynab_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// allSentinels enumerates every exported sentinel so the taxonomy test can
// assert "and nothing else".
func allSentinels() map[string]error {
	return map[string]error{
		"ErrBadRequest":         ynab.ErrBadRequest,
		"ErrNotAuthorized":      ynab.ErrNotAuthorized,
		"ErrForbidden":          ynab.ErrForbidden,
		"ErrSubscriptionLapsed": ynab.ErrSubscriptionLapsed,
		"ErrTrialExpired":       ynab.ErrTrialExpired,
		"ErrUnauthorizedScope":  ynab.ErrUnauthorizedScope,
		"ErrDataLimitReached":   ynab.ErrDataLimitReached,
		"ErrNotFound":           ynab.ErrNotFound,
		"ErrRouteNotFound":      ynab.ErrRouteNotFound,
		"ErrResourceNotFound":   ynab.ErrResourceNotFound,
		"ErrConflict":           ynab.ErrConflict,
		"ErrRateLimited":        ynab.ErrRateLimited,
		"ErrServerError":        ynab.ErrServerError,
		"ErrServiceUnavailable": ynab.ErrServiceUnavailable,
	}
}

func TestErrorTaxonomy(t *testing.T) {
	t.Parallel()

	// The full 12-row wire taxonomy: each {status, id} matches its specific
	// sentinel AND its class sentinel, and nothing else.
	tests := []struct {
		status int
		id     string
		name   string
		want   []string
	}{
		{status: 400, id: "400", name: "bad_request", want: []string{"ErrBadRequest"}},
		{status: 401, id: "401", name: "not_authorized", want: []string{"ErrNotAuthorized"}},
		{status: 403, id: "403.1", name: "subscription_lapsed", want: []string{"ErrSubscriptionLapsed", "ErrForbidden"}},
		{status: 403, id: "403.2", name: "trial_expired", want: []string{"ErrTrialExpired", "ErrForbidden"}},
		{status: 403, id: "403.3", name: "unauthorized_scope", want: []string{"ErrUnauthorizedScope", "ErrForbidden"}},
		{status: 403, id: "403.4", name: "data_limit_reached", want: []string{"ErrDataLimitReached", "ErrForbidden"}},
		{status: 404, id: "404.1", name: "not_found", want: []string{"ErrRouteNotFound", "ErrNotFound"}},
		{status: 404, id: "404.2", name: "resource_not_found", want: []string{"ErrResourceNotFound", "ErrNotFound"}},
		{status: 409, id: "409", name: "conflict", want: []string{"ErrConflict"}},
		{status: 429, id: "429", name: "too_many_requests", want: []string{"ErrRateLimited"}},
		{status: 500, id: "500", name: "internal_server_error", want: []string{"ErrServerError"}},
		{status: 503, id: "503", name: "service_unavailable", want: []string{"ErrServiceUnavailable"}},
	}
	for _, tt := range tests {
		t.Run(tt.id+"_"+tt.name, func(t *testing.T) {
			t.Parallel()

			// Wrapped once, as transport will surface it.
			err := fmt.Errorf("get user: %w", &ynab.Error{StatusCode: tt.status, ID: tt.id, Name: tt.name, Detail: "d"})

			want := map[string]bool{}
			for _, n := range tt.want {
				want[n] = true
			}
			for name, sentinel := range allSentinels() {
				require.Equal(t, want[name], errors.Is(err, sentinel),
					"%s (%s) vs %s", tt.id, tt.name, name)
			}
		})
	}

	t.Run("unknown sub-id still matches its class", func(t *testing.T) {
		t.Parallel()

		err := &ynab.Error{StatusCode: 403, ID: "403.9", Name: "brand_new"}
		require.ErrorIs(t, err, ynab.ErrForbidden)
		require.NotErrorIs(t, err, ynab.ErrSubscriptionLapsed)

		err = &ynab.Error{StatusCode: 404, ID: "", Name: ""}
		require.ErrorIs(t, err, ynab.ErrNotFound)
		require.NotErrorIs(t, err, ynab.ErrResourceNotFound)
	})
}

func TestErrorNilSafety(t *testing.T) {
	t.Parallel()

	var e *ynab.Error
	require.NotPanics(t, func() { _ = e.Error() })
	require.NotEmpty(t, e.Error())

	require.NotPanics(t, func() {
		require.NotErrorIs(t, error(e), ynab.ErrNotFound)
	})
	require.NotPanics(t, func() {
		var nilTarget error
		require.NotErrorIs(t, &ynab.Error{StatusCode: 404, ID: "404.2"}, nilTarget)
	})

	var a *ynab.ArgumentError
	require.NotPanics(t, func() { _ = a.Error() })
	require.NotEmpty(t, a.Error())
}

func TestErrorMessages(t *testing.T) {
	t.Parallel()

	e := &ynab.Error{StatusCode: 403, ID: "403.1", Name: "subscription_lapsed", Detail: "Subscription lapsed"}
	require.Equal(t, "ynab: HTTP 403 403.1 subscription_lapsed: Subscription lapsed", e.Error())

	// Undecodable body: only the status is known.
	require.Equal(t, "ynab: HTTP 500", (&ynab.Error{StatusCode: 500}).Error())

	a := &ynab.ArgumentError{Op: "Categories.Assign", Field: "month", Reason: "month must not be zero"}
	require.Equal(t, "ynab: Categories.Assign: month: month must not be zero", a.Error())

	// Field "" means a cross-field violation.
	cross := &ynab.ArgumentError{Op: "Transactions.Create", Reason: "split amounts must sum to Amount"}
	require.Equal(t, "ynab: Transactions.Create: split amounts must sum to Amount", cross.Error())
}

// fakeNetError stands in for a transport-layer net.Error.
type fakeNetError struct{ timeout bool }

func (e *fakeNetError) Error() string   { return "fake net error" }
func (e *fakeNetError) Timeout() bool   { return e.timeout }
func (e *fakeNetError) Temporary() bool { return false }

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	wrap := func(err error) error { return fmt.Errorf("attempt 1: %w", err) }

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "429", err: &ynab.Error{StatusCode: 429, ID: "429"}, want: true},
		{name: "500", err: &ynab.Error{StatusCode: 500, ID: "500"}, want: true},
		{name: "503 wrapped", err: wrap(&ynab.Error{StatusCode: 503, ID: "503"}), want: true},
		{name: "400", err: &ynab.Error{StatusCode: 400, ID: "400"}, want: false},
		{name: "403.1", err: &ynab.Error{StatusCode: 403, ID: "403.1"}, want: false},
		{name: "404.2 wrapped", err: wrap(&ynab.Error{StatusCode: 404, ID: "404.2"}), want: false},
		{name: "409", err: &ynab.Error{StatusCode: 409, ID: "409"}, want: false},
		{name: "argument error", err: &ynab.ArgumentError{Op: "op", Reason: "r"}, want: false},
		{name: "argument error wrapped", err: wrap(&ynab.ArgumentError{Op: "op", Reason: "r"}), want: false},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "context canceled wrapped in net error", err: wrap(&wrappingNetError{err: context.Canceled}), want: false},
		{name: "bare caller deadline", err: context.DeadlineExceeded, want: false},
		{name: "wrapped bare caller deadline", err: wrap(context.DeadlineExceeded), want: false},
		{name: "transport timeout (net.Error, Timeout true, wraps deadline)", err: wrap(&wrappingNetError{err: context.DeadlineExceeded, timeout: true}), want: true},
		{name: "connection failure (net.Error, Timeout false)", err: wrap(&fakeNetError{timeout: false}), want: true},
		{name: "plain error", err: errors.New("boom"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, ynab.IsRetryable(tt.err))
		})
	}
}

// wrappingNetError is a net.Error that wraps an inner error, like
// *url.Error wraps context errors on in-flight failures.
type wrappingNetError struct {
	err     error
	timeout bool
}

func (e *wrappingNetError) Error() string   { return "net: " + e.err.Error() }
func (e *wrappingNetError) Unwrap() error   { return e.err }
func (e *wrappingNetError) Timeout() bool   { return e.timeout }
func (e *wrappingNetError) Temporary() bool { return false }

func TestErrorRetryAfter(t *testing.T) {
	t.Parallel()

	// RetryAfter zero means unknown — never "retry immediately".
	e := &ynab.Error{StatusCode: 429, ID: "429", RetryAfter: 30 * time.Second}
	require.Equal(t, 30*time.Second, e.RetryAfter)
	require.Zero(t, (&ynab.Error{StatusCode: 429}).RetryAfter)
}
