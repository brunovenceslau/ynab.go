// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package transport

// White-box tests for pieces with no black-box observation path: the
// hookError plumbing, the backoff arithmetic at extreme attempt indices,
// and the Retry-After parser's date branches.

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHookError(t *testing.T) {
	t.Parallel()

	inner := errors.New("limiter says no")
	he := &hookError{err: inner}
	require.Equal(t, "limiter says no", he.Error())
	require.ErrorIs(t, he, inner, "Unwrap exposes the wrapped hook failure")
}

func TestRetryDelayBounds(t *testing.T) {
	t.Parallel()

	core := func(minB, maxB time.Duration) *Core {
		return &Core{
			Retry: RetryConfig{MinBackoff: minB, MaxBackoff: maxB},
			Rand:  func() float64 { return 1 },
		}
	}

	t.Run("doubling caps at MaxBackoff", func(t *testing.T) {
		t.Parallel()

		c := core(time.Second, 30*time.Second)
		require.Equal(t, 2*time.Second, c.retryDelay(1, result{}))
		require.Equal(t, 30*time.Second, c.retryDelay(5, result{}), "1s<<5 = 32s caps at 30s")
	})

	t.Run("huge attempt index never overflows back to MinBackoff", func(t *testing.T) {
		t.Parallel()

		// The i>=34 shift used to wrap negative and collapse the delay to
		// MinBackoff — retries got MORE aggressive exactly when they
		// should not.
		c := core(time.Second, 30*time.Second)
		for _, i := range []int{34, 62, 63, 64, 1000} {
			require.Equal(t, 30*time.Second, c.retryDelay(i, result{}), "i=%d", i)
		}
	})

	t.Run("large MinBackoff does not overflow either", func(t *testing.T) {
		t.Parallel()

		c := core(10*time.Second, 25*time.Second)
		for _, i := range []int{30, 40, 63} {
			require.Equal(t, 25*time.Second, c.retryDelay(i, result{}), "i=%d", i)
		}
	})

	t.Run("retry-after within cap is honored as given", func(t *testing.T) {
		t.Parallel()

		c := core(time.Millisecond, 2*time.Millisecond)
		res := result{status: 429, header: map[string][]string{"Retry-After": {"30"}}}
		require.Equal(t, 30*time.Second, c.retryDelay(1, res), "within the 1-minute floor cap")
	})
}

func TestSleeperZeroDuration(t *testing.T) {
	t.Parallel()

	s := &sleeper{}
	defer s.stop()
	require.NoError(t, s.sleep(t.Context(), 0), "non-positive sleep returns immediately")
	require.NoError(t, s.sleep(t.Context(), -time.Second))
}

func TestRetryAfterDelayForms(t *testing.T) {
	t.Parallel()

	require.Equal(t, 30*time.Second, RetryAfterDelay("30"))
	require.Zero(t, RetryAfterDelay(""))
	require.Zero(t, RetryAfterDelay("soon-ish"))
	require.Zero(t, RetryAfterDelay("-5"), "negative seconds mean unknown")

	future := time.Now().Add(90 * time.Second).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	d := RetryAfterDelay(future)
	require.Positive(t, d)
	require.LessOrEqual(t, d, 90*time.Second)

	past := time.Now().Add(-time.Hour).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	require.Zero(t, RetryAfterDelay(past), "a date in the past means no wait, not a negative one")
}
