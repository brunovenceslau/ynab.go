// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

// Package transport implements the wire layer: request building, the
// data/error envelope split, and the retry pipeline. It operates on bytes
// and status codes only — YNAB error semantics live in the root package and
// are injected via Core.DecodeError, which keeps the public error types in
// the root without a root↔transport import cycle.
//
// Every error wrap in this package uses %w: the root package's documented
// errors.Is/errors.As chain guarantees depend on it.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Core carries the immutable wire configuration the root Client wires at
// construction. Fields are never mutated after that.
type Core struct {
	HTTPClient *http.Client
	BaseURL    *url.URL
	UserAgent  string

	// Token supplies the bearer token, consulted once per attempt.
	Token func(ctx context.Context) (string, error)

	// Wait is the proactive limiter hook, run before every attempt.
	// Nil means no limiter.
	Wait func(ctx context.Context) error

	// Timeout bounds each attempt via context.WithTimeout.
	Timeout time.Duration

	// DecodeError maps a non-2xx response to the root package's error
	// types. It is the only reader of response headers besides the retry
	// pipeline's Retry-After peek.
	DecodeError func(status int, body []byte, hdr http.Header) error

	// Retry configures the pipeline in retry.go.
	Retry RetryConfig

	// MaxResponseBytes bounds how much of a response body is read into
	// memory (hostile-server hardening). Zero means the 128 MiB default —
	// far above any real YNAB payload, full-plan exports included.
	MaxResponseBytes int64

	// Rand overrides the backoff jitter source; tests inject a
	// deterministic one. Nil means math/rand/v2.
	Rand func() float64

	Logger *slog.Logger
}

// Do executes one API operation and decodes the success envelope's data
// payload into T. Non-2xx responses are mapped through Core.DecodeError.
// No response header is ever read on the success path.
func Do[T any](ctx context.Context, c *Core, method, path string, query url.Values, body any) (T, error) {
	var zero T

	payload, err := marshalBody(body)
	if err != nil {
		return zero, fmt.Errorf("ynab: %s %s: encode request body: %w", method, path, err)
	}

	res, err := c.execute(ctx, method, path, query, payload)
	if err != nil {
		return zero, err
	}
	if res.status < 200 || res.status > 299 {
		return zero, c.DecodeError(res.status, res.body, res.header)
	}
	v, err := decodeData[T](res.body)
	if err != nil {
		return zero, fmt.Errorf("ynab: %s %s: decode response: %w", method, path, err)
	}
	return v, nil
}

// DoRaw executes one operation and returns the raw response body bytes —
// no envelope semantics on either side. Non-2xx responses still map
// through Core.DecodeError; the full retry/limiter/token pipeline applies.
func DoRaw(ctx context.Context, c *Core, method, path string, query url.Values, payload []byte) ([]byte, error) {
	res, err := c.execute(ctx, method, path, query, payload)
	if err != nil {
		return nil, err
	}
	if res.status < 200 || res.status > 299 {
		return nil, c.DecodeError(res.status, res.body, res.header)
	}
	return res.body, nil
}

// result is one attempt's raw outcome.
type result struct {
	status int
	body   []byte
	header http.Header
}

// attempt performs a single wire attempt: limiter wait, token fetch, then
// the request, all under the per-attempt timeout.
func (c *Core) attempt(ctx context.Context, method, path string, query url.Values, payload []byte) (result, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	if c.Wait != nil {
		if err := c.Wait(ctx); err != nil {
			return result{}, &hookError{err: fmt.Errorf("ynab: %s %s: limiter: %w", method, path, err)}
		}
	}
	token, err := c.Token(ctx)
	if err != nil {
		return result{}, &hookError{err: fmt.Errorf("ynab: %s %s: token source: %w", method, path, err)}
	}

	u := c.requestURL(path, query)
	req, err := http.NewRequestWithContext(ctx, method, u, requestBody(payload))
	if err != nil {
		return result{}, fmt.Errorf("ynab: %s %s: build request: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token) // Set, never Add: exactly one credential header
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.Logger.DebugContext(ctx, "ynab: request", "method", method, "url", u)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return result{}, fmt.Errorf("ynab: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	limit := c.MaxResponseBytes
	if limit <= 0 {
		limit = defaultMaxResponseBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return result{}, fmt.Errorf("ynab: %s %s: read response: %w", method, path, err)
	}
	if int64(len(body)) > limit {
		return result{}, fmt.Errorf("ynab: %s %s: response body exceeds %d bytes", method, path, limit)
	}

	c.Logger.DebugContext(ctx, "ynab: response", "method", method, "url", u, "status", resp.StatusCode)

	return result{status: resp.StatusCode, body: body, header: resp.Header}, nil
}

// defaultMaxResponseBytes is the response-size ceiling when the Core does
// not set one.
const defaultMaxResponseBytes = 128 << 20

// requestURL joins the base URL, the (already escaped) operation path, and
// the query string. Dynamic path segments must come through JoinPath.
func (c *Core) requestURL(path string, query url.Values) string {
	s := strings.TrimSuffix(c.BaseURL.String(), "/") + "/" + strings.TrimPrefix(path, "/")
	if enc := query.Encode(); enc != "" {
		s += "?" + enc
	}
	return s
}

// JoinPath builds an operation path from segments, percent-escaping each
// one so a caller-supplied ID can never traverse into another route. The
// dot segments "." and ".." are escaped entirely — servers normalize raw
// dot-segments before decoding, escaped ones they do not.
func JoinPath(segments ...string) string {
	escaped := make([]string, len(segments))
	for i, s := range segments {
		switch s {
		case ".":
			escaped[i] = "%2E"
		case "..":
			escaped[i] = "%2E%2E"
		default:
			escaped[i] = url.PathEscape(s)
		}
	}
	return strings.Join(escaped, "/")
}

// marshalBody encodes a non-nil body as JSON.
func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return json.Marshal(body)
}

// requestBody wraps the payload for http.NewRequestWithContext, keeping
// bodyless requests truly bodyless.
func requestBody(payload []byte) io.Reader {
	if payload == nil {
		return nil
	}
	return bytes.NewReader(payload)
}
