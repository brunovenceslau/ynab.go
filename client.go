// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"pkg.venceslau.dev/ynab/internal/transport"
)

// Version is this library's release version, as sent in the
// User-Agent header.
const Version = "1.0.0"

// defaultBaseURL is the documented YNAB API root.
const defaultBaseURL = "https://api.ynab.com/v1"

// defaultTimeout is the per-attempt timeout: YNAB cuts requests off
// server-side at 30 seconds, so waiting longer only delays the 503.
const defaultTimeout = 30 * time.Second

// defaultRetry is the retry policy every client starts with; zero
// fields passed to WithRetryPolicy fall back to it field by field.
var defaultRetry = RetryPolicy{
	MaxAttempts: 3,
	MinBackoff:  time.Second,
	MaxBackoff:  30 * time.Second,
	RetryWrites: false,
}

// TokenSource supplies the bearer token for each attempt. Implement it for
// OAuth flows whose access tokens expire (YNAB's expire after two hours);
// it is consulted per attempt, so a refreshed token is picked up mid-retry.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// Limiter gates request attempts proactively: Wait runs before every
// attempt, including retries. *golang.org/x/time/rate.Limiter satisfies it;
// no dependency on that module is taken.
type Limiter interface {
	Wait(ctx context.Context) error
}

// RetryPolicy configures the retry pipeline.
type RetryPolicy struct {
	MaxAttempts int           // default 3
	MinBackoff  time.Duration // default 1s
	MaxBackoff  time.Duration // default 30s; backoff uses full jitter
	RetryWrites bool          // default false: 500/503/transport retried only for GET/DELETE
}

// Client is a YNAB API client. It is safe for concurrent use: all
// configuration is set once at construction and never mutated afterwards.
// The zero Client is unusable by design — obtain one from New or
// NewWithTokenSource.
type Client struct {
	tokenSource TokenSource
	httpClient  *http.Client
	baseURL     *url.URL
	userAgent   string
	timeout     time.Duration
	retry       RetryPolicy
	retryOff    bool
	limiter     Limiter
	logger      *slog.Logger

	// configErr is the first option failure; every method surfaces it as
	// *ArgumentError before any I/O (the config-error contract).
	configErr error

	// core is the wired transport layer, built once after options run.
	core *transport.Core
}

// staticToken adapts a fixed personal-access token to TokenSource.
type staticToken string

// Token returns the fixed token; it never fails.
func (t staticToken) Token(context.Context) (string, error) { return string(t), nil }

// New returns a Client authenticating with a fixed personal-access token.
// Failable options never make New return an error: the first option
// failure is stored and surfaced by every method as *ArgumentError before
// any I/O.
func New(token string, opts ...Option) *Client {
	return NewWithTokenSource(staticToken(token), opts...)
}

// NewWithTokenSource returns a Client that asks ts for a bearer token
// before every attempt — the constructor for OAuth tokens that expire.
// The option-failure contract of New applies unchanged; a nil ts trips it
// too instead of panicking.
func NewWithTokenSource(ts TokenSource, opts ...Option) *Client {
	var configErr error
	if ts == nil {
		ts = staticToken("")
		configErr = &ArgumentError{Op: "ynab.NewWithTokenSource", Field: "ts", Reason: "token source must not be nil"}
	}
	c := &Client{
		tokenSource: ts,
		configErr:   configErr,
		httpClient:  http.DefaultClient,
		userAgent:   "pkg.venceslau.dev/ynab/" + Version,
		timeout:     defaultTimeout,
		retry:       defaultRetry,
		logger:      slog.New(slog.DiscardHandler),
	}
	c.baseURL, _ = url.Parse(defaultBaseURL) // a constant; cannot fail
	for _, opt := range opts {
		if opt.apply != nil {
			opt.apply(c)
		}
	}

	c.core = &transport.Core{
		HTTPClient:  c.httpClient,
		BaseURL:     c.baseURL,
		UserAgent:   c.userAgent,
		Token:       c.tokenSource.Token,
		Timeout:     c.timeout,
		DecodeError: decodeWireError,
		Retry: transport.RetryConfig{
			MaxAttempts: c.retry.MaxAttempts,
			MinBackoff:  c.retry.MinBackoff,
			MaxBackoff:  c.retry.MaxBackoff,
			RetryWrites: c.retry.RetryWrites,
			Disabled:    c.retryOff,
		},
		Logger: c.logger,
	}
	if c.limiter != nil {
		c.core.Wait = c.limiter.Wait
	}
	return c
}

// Option configures a Client at construction. Options run in order; the
// first failing option wins the config-error contract. The zero Option
// is inert.
type Option struct {
	apply func(*Client)
}

// WithHTTPClient replaces the underlying *http.Client (default
// http.DefaultClient). Use it to install transports, proxies, or fakes.
func WithHTTPClient(hc *http.Client) Option {
	return Option{apply: func(c *Client) {
		if hc == nil {
			c.storeConfigErr("WithHTTPClient", "client must not be nil")
			return
		}
		c.httpClient = hc
	}}
}

// WithBaseURL points the client at a different API root (default
// https://api.ynab.com/v1) — the first-class seam for httptest servers.
// A rawURL that is not an absolute http(s) URL, or that carries
// credentials, a query, or a fragment, never falls back silently: it
// trips the config-error contract instead. Plain http is accepted for
// local test servers only — over a real network it would send the bearer
// token in cleartext.
func WithBaseURL(rawURL string) Option {
	return Option{apply: func(c *Client) {
		u, err := url.Parse(rawURL)
		switch {
		case err != nil || !u.IsAbs() || u.Host == "":
			c.storeConfigErr("WithBaseURL", "not an absolute URL: "+rawURL)
		case u.Scheme != "http" && u.Scheme != "https":
			c.storeConfigErr("WithBaseURL", "scheme must be http or https: "+rawURL)
		case u.User != nil:
			c.storeConfigErr("WithBaseURL", "URL must not carry credentials")
		case u.RawQuery != "" || u.Fragment != "":
			c.storeConfigErr("WithBaseURL", "URL must not carry a query or fragment: "+rawURL)
		default:
			c.baseURL = u
		}
	}}
}

// WithUserAgent replaces the default User-Agent
// "pkg.venceslau.dev/ynab/<Version>" — the module path is the library's
// canonical identity in the Go ecosystem.
func WithUserAgent(ua string) Option {
	return Option{apply: func(c *Client) { c.userAgent = ua }}
}

// WithTimeout sets the per-attempt timeout (default 30s, YNAB's own
// server-side cutoff). Each attempt runs under
// context.WithTimeout(callerCtx, d); the caller's context still bounds the
// whole call.
func WithTimeout(d time.Duration) Option {
	return Option{apply: func(c *Client) {
		if d <= 0 {
			c.storeConfigErr("WithTimeout", "timeout must be positive")
			return
		}
		c.timeout = d
	}}
}

// WithRetryPolicy replaces the retry policy. Zero fields keep their
// defaults (MaxAttempts 3, MinBackoff 1s, MaxBackoff 30s, RetryWrites
// false), so setting a single field is safe.
func WithRetryPolicy(p RetryPolicy) Option {
	return Option{apply: func(c *Client) {
		if p.MaxAttempts == 0 {
			p.MaxAttempts = defaultRetry.MaxAttempts
		}
		if p.MinBackoff == 0 {
			p.MinBackoff = defaultRetry.MinBackoff
		}
		if p.MaxBackoff == 0 {
			p.MaxBackoff = defaultRetry.MaxBackoff
		}
		if p.MaxAttempts < 1 {
			c.storeConfigErr("WithRetryPolicy", "MaxAttempts must be at least 1")
			return
		}
		if p.MinBackoff <= 0 || p.MaxBackoff < p.MinBackoff {
			c.storeConfigErr("WithRetryPolicy", "want 0 < MinBackoff <= MaxBackoff")
			return
		}
		c.retry = p
	}}
}

// WithRetryDisabled short-circuits the retry pipeline: every call makes
// exactly one attempt. Pair with [IsRetryable] to orchestrate custom loops.
func WithRetryDisabled() Option {
	return Option{apply: func(c *Client) { c.retryOff = true }}
}

// WithLimiter installs a proactive rate limiter: Wait runs before every
// attempt, including retries.
func WithLimiter(l Limiter) Option {
	return Option{apply: func(c *Client) {
		if l == nil {
			c.storeConfigErr("WithLimiter", "limiter must not be nil")
			return
		}
		c.limiter = l
	}}
}

// WithLogger enables slog.DebugContext tracing of requests and responses.
// The Authorization header is always redacted; errors are logged or
// returned, never both.
func WithLogger(l *slog.Logger) Option {
	return Option{apply: func(c *Client) {
		if l == nil {
			c.storeConfigErr("WithLogger", "logger must not be nil")
			return
		}
		c.logger = l
	}}
}

// RawDo executes an arbitrary API operation and returns the raw response
// body bytes: method, an already-escaped path relative to the base URL
// (build dynamic segments with care), optional query values, and an
// optional raw JSON body. The call flows through the full
// limiter/token/retry pipeline and non-2xx responses map to *Error, but
// no envelope semantics apply on either side.
//
// EXPERIMENTAL: RawDo exists as the escape hatch for endpoints this
// library does not cover yet. It is outside the compatibility promise —
// its signature and behavior may change in any release.
//
// Escape dynamic path segments yourself and beware the trap:
// url.PathEscape("..") returns ".." unchanged, so interpolating untrusted
// input can traverse onto another route. Never build RawDo paths from
// end-user input.
func (c *Client) RawDo(ctx context.Context, method, path string, q url.Values, body []byte) ([]byte, error) {
	if err := c.configError(); err != nil {
		return nil, err
	}
	return transport.DoRaw(ctx, c.core, method, path, q, body)
}

// storeConfigErr records the first option failure; later failures keep the
// first (it names the option the caller must fix first).
func (c *Client) storeConfigErr(option, reason string) {
	if c.configErr == nil {
		c.configErr = &ArgumentError{Op: "ynab.New", Field: option, Reason: reason}
	}
}

// configError returns the stored first option failure, if any. Every I/O
// method calls it before touching the network.
func (c *Client) configError() error {
	return c.configErr
}
