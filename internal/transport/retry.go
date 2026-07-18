// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package transport

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// hookError marks a limiter or token-source failure: the attempt never
// reached the wire and the pipeline must abort rather than retry.
type hookError struct {
	err error
}

func (e *hookError) Error() string { return e.err.Error() }
func (e *hookError) Unwrap() error { return e.err }

// RetryConfig is the pipeline configuration the root Client maps from its
// public RetryPolicy.
type RetryConfig struct {
	MaxAttempts int
	MinBackoff  time.Duration
	MaxBackoff  time.Duration
	RetryWrites bool
	Disabled    bool
}

// execute runs the frozen retry pipeline: per attempt Wait → Token → send
// under the per-attempt timeout, then classify, then a context-aware
// full-jitter backoff. 429 is retried on any verb; 500/503 and transport
// failures only for GET/DELETE unless RetryWrites.
func (c *Core) execute(ctx context.Context, method, path string, query url.Values, payload []byte) (result, error) {
	attempts := c.Retry.MaxAttempts
	if c.Retry.Disabled || attempts < 1 {
		attempts = 1
	}

	slp := &sleeper{}
	defer slp.stop()

	var res result
	var sendErr error
	for i := range attempts {
		if i > 0 {
			if err := slp.sleep(ctx, c.retryDelay(i, res)); err != nil {
				return result{}, fmt.Errorf("ynab: %s %s: canceled during backoff: %w", method, path, err)
			}
		}

		res, sendErr = c.attempt(ctx, method, path, query, payload)
		retry, abortErr := c.classify(ctx, method, res, sendErr)
		if abortErr != nil {
			return result{}, abortErr
		}
		if !retry {
			return res, nil
		}
	}

	if sendErr != nil {
		return result{}, sendErr
	}
	return res, nil
}

// classify decides one attempt's fate: abort with an error, retry, or done.
func (c *Core) classify(ctx context.Context, method string, res result, sendErr error) (retry bool, abortErr error) {
	if sendErr != nil {
		var hookErr *hookError
		switch {
		case errors.As(sendErr, &hookErr):
			return false, hookErr.err // limiter/token failures abort the pipeline
		case ctx.Err() != nil:
			return false, sendErr // the caller's own context ended; retrying is pointless
		case !c.retryVerb(method):
			return false, sendErr // a timed-out write may have applied
		}
		return true, nil // transport failure on a retryable verb
	}

	switch res.status {
	case http.StatusTooManyRequests:
		// 429 was rejected before processing — safe on any verb. But when
		// the server asks for a wait beyond what this pipeline will sleep,
		// surface the decoded 429 instead: Error.RetryAfter carries the
		// server's number and the caller decides.
		if d := RetryAfterDelay(res.header.Get("Retry-After")); d > c.retryAfterCap() {
			return false, nil
		}
		return true, nil
	case http.StatusInternalServerError, http.StatusServiceUnavailable:
		return c.retryVerb(method), nil
	}
	return false, nil // success or a terminal status for the caller to decode
}

// retryAfterCap bounds how long an honored Retry-After may park the
// pipeline: the configured MaxBackoff, but never below one minute (YNAB's
// documented waits are shorter; its rolling window refills continuously).
func (c *Core) retryAfterCap() time.Duration {
	return max(c.Retry.MaxBackoff, time.Minute)
}

// retryVerb reports whether transport failures and 500/503 may be retried
// for this verb.
func (c *Core) retryVerb(method string) bool {
	return c.Retry.RetryWrites || method == http.MethodGet || method == http.MethodDelete
}

// retryDelay computes the wait before retry i (1-based). A 429's
// Retry-After is honored best-effort — never required: without it, the
// i-th retry waits a full-jittered duration in the half-open interval
// [MinBackoff, min(MaxBackoff, MinBackoff·2^i)).
func (c *Core) retryDelay(i int, last result) time.Duration {
	if last.status == http.StatusTooManyRequests {
		if d := RetryAfterDelay(last.header.Get("Retry-After")); d > 0 {
			return min(d, c.retryAfterCap())
		}
	}

	// The overflow-free doubling test: MinBackoff·2^i ≤ MaxBackoff iff
	// MinBackoff ≤ MaxBackoff>>i. Once the doubling passes the cap (or i
	// is beyond any shiftable range), the upper bound is MaxBackoff.
	upper := c.Retry.MaxBackoff
	if i < 63 && c.Retry.MinBackoff <= c.Retry.MaxBackoff>>uint(i) {
		upper = c.Retry.MinBackoff << uint(i)
	}
	if upper <= c.Retry.MinBackoff {
		return c.Retry.MinBackoff
	}
	return c.Retry.MinBackoff + time.Duration(c.rand()*float64(upper-c.Retry.MinBackoff))
}

// rand returns the jitter source, defaulting to math/rand/v2.
func (c *Core) rand() float64 {
	if c.Rand != nil {
		return c.Rand()
	}
	return rand.Float64()
}

// RetryAfterDelay parses the two documented Retry-After forms — delay
// seconds and HTTP-date — returning 0 (unknown) for anything else. The
// root package reuses it when decoding Error.RetryAfter, so the two
// readers of the header can never drift.
func RetryAfterDelay(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// sleeper is a reused backoff timer whose sleep is always cancelable: it
// selects on ctx.Done, so cancellation during backoff returns promptly.
// go.mod declares Go 1.24, so the 1.23+ timer semantics apply — Stop and
// Reset need no channel drain (draining would block on the unbuffered
// timer channel).
type sleeper struct {
	timer *time.Timer
}

func (s *sleeper) sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	if s.timer == nil {
		s.timer = time.NewTimer(d)
	} else {
		s.timer.Reset(d)
	}
	select {
	case <-ctx.Done():
		s.timer.Stop()
		return ctx.Err()
	case <-s.timer.C:
		return nil
	}
}

func (s *sleeper) stop() {
	if s.timer != nil {
		s.timer.Stop()
	}
}
