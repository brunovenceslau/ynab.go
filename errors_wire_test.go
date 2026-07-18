package ynab_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

func TestDecodeWireError(t *testing.T) {
	t.Parallel()

	t.Run("full envelope", func(t *testing.T) {
		t.Parallel()

		err := ynab.DecodeWireError(401, []byte(`{"error":{"id":"401","name":"not_authorized","detail":"Unauthorized"}}`), http.Header{})

		var apiErr *ynab.Error
		require.ErrorAs(t, err, &apiErr)
		require.Equal(t, 401, apiErr.StatusCode)
		require.Equal(t, "401", apiErr.ID)
		require.Equal(t, "not_authorized", apiErr.Name)
		require.Equal(t, "Unauthorized", apiErr.Detail)
		require.ErrorIs(t, err, ynab.ErrNotAuthorized)
	})

	t.Run("sub-coded id keeps class matching", func(t *testing.T) {
		t.Parallel()

		err := ynab.DecodeWireError(404, []byte(`{"error":{"id":"404.2","name":"resource_not_found","detail":"d"}}`), http.Header{})
		require.ErrorIs(t, err, ynab.ErrResourceNotFound)
		require.ErrorIs(t, err, ynab.ErrNotFound)
	})

	t.Run("garbage body still yields the status", func(t *testing.T) {
		t.Parallel()

		err := ynab.DecodeWireError(500, []byte(`<html>boom`), http.Header{})

		var apiErr *ynab.Error
		require.ErrorAs(t, err, &apiErr)
		require.Equal(t, 500, apiErr.StatusCode)
		require.Empty(t, apiErr.ID)
		require.ErrorIs(t, err, ynab.ErrServerError)
	})

	t.Run("retry-after in seconds", func(t *testing.T) {
		t.Parallel()

		hdr := http.Header{}
		hdr.Set("Retry-After", "30")
		err := ynab.DecodeWireError(429, []byte(`{"error":{"id":"429","name":"too_many_requests","detail":"d"}}`), hdr)

		var apiErr *ynab.Error
		require.ErrorAs(t, err, &apiErr)
		require.Equal(t, 30*time.Second, apiErr.RetryAfter)
	})

	t.Run("retry-after as http date", func(t *testing.T) {
		t.Parallel()

		hdr := http.Header{}
		hdr.Set("Retry-After", time.Now().Add(90*time.Second).UTC().Format(http.TimeFormat))
		err := ynab.DecodeWireError(429, nil, hdr)

		var apiErr *ynab.Error
		require.ErrorAs(t, err, &apiErr)
		require.Positive(t, apiErr.RetryAfter)
		require.LessOrEqual(t, apiErr.RetryAfter, 90*time.Second)
	})

	t.Run("missing retry-after means unknown, never immediate", func(t *testing.T) {
		t.Parallel()

		// The issue#38 seed: a 429 without Retry-After must still decode.
		err := ynab.DecodeWireError(429, []byte(`{"error":{"id":"429","name":"too_many_requests","detail":"d"}}`), http.Header{})

		var apiErr *ynab.Error
		require.ErrorAs(t, err, &apiErr)
		require.Zero(t, apiErr.RetryAfter)
		require.ErrorIs(t, err, ynab.ErrRateLimited)
	})

	t.Run("malformed retry-after ignored", func(t *testing.T) {
		t.Parallel()

		hdr := http.Header{}
		hdr.Set("Retry-After", "soon-ish")
		err := ynab.DecodeWireError(429, nil, hdr)

		var apiErr *ynab.Error
		require.ErrorAs(t, err, &apiErr)
		require.Zero(t, apiErr.RetryAfter)
	})
}
