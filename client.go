package ynab

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"pkg.venceslau.dev/ynab/internal/transport"
)

// Version is this library's release version. The release gate asserts it
// equals the git tag being published.
const Version = "1.0.0"

// defaultBaseURL is the documented YNAB API root.
const defaultBaseURL = "https://api.ynab.com/v1"

// defaultTimeout is the per-attempt timeout: YNAB cuts requests off
// server-side at 30 seconds, so waiting longer only delays the 503.
const defaultTimeout = 30 * time.Second

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
// The option-failure contract of New applies unchanged.
func NewWithTokenSource(ts TokenSource, opts ...Option) *Client {
	c := &Client{
		tokenSource: ts,
		httpClient:  http.DefaultClient,
		userAgent:   "ynab.go/" + Version,
		timeout:     defaultTimeout,
		retry: RetryPolicy{
			MaxAttempts: 3,
			MinBackoff:  time.Second,
			MaxBackoff:  30 * time.Second,
			RetryWrites: false,
		},
		logger: slog.New(slog.DiscardHandler),
	}
	c.baseURL, _ = url.Parse(defaultBaseURL) // a constant; cannot fail
	for _, opt := range opts {
		opt(c)
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
// first failing option wins the config-error contract.
type Option func(*Client)

// WithHTTPClient replaces the underlying *http.Client (default
// http.DefaultClient). Use it to install transports, proxies, or fakes.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc == nil {
			c.storeConfigErr("WithHTTPClient", "client must not be nil")
			return
		}
		c.httpClient = hc
	}
}

// WithBaseURL points the client at a different API root (default
// https://api.ynab.com/v1) — the first-class seam for httptest servers.
// A rawURL that does not parse as an absolute URL never falls back
// silently: it trips the config-error contract instead.
func WithBaseURL(rawURL string) Option {
	return func(c *Client) {
		u, err := url.Parse(rawURL)
		if err != nil || !u.IsAbs() || u.Host == "" {
			c.storeConfigErr("WithBaseURL", "not an absolute URL: "+rawURL)
			return
		}
		c.baseURL = u
	}
}

// WithUserAgent replaces the default User-Agent "ynab.go/<Version>".
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithTimeout sets the per-attempt timeout (default 30s, YNAB's own
// server-side cutoff). Each attempt runs under
// context.WithTimeout(callerCtx, d); the caller's context still bounds the
// whole call.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d <= 0 {
			c.storeConfigErr("WithTimeout", "timeout must be positive")
			return
		}
		c.timeout = d
	}
}

// WithRetryPolicy replaces the default retry policy
// {MaxAttempts: 3, MinBackoff: 1s, MaxBackoff: 30s, RetryWrites: false}.
func WithRetryPolicy(p RetryPolicy) Option {
	return func(c *Client) {
		if p.MaxAttempts < 1 {
			c.storeConfigErr("WithRetryPolicy", "MaxAttempts must be at least 1")
			return
		}
		if p.MinBackoff < 0 || p.MaxBackoff < p.MinBackoff {
			c.storeConfigErr("WithRetryPolicy", "want 0 <= MinBackoff <= MaxBackoff")
			return
		}
		c.retry = p
	}
}

// WithRetryDisabled short-circuits the retry pipeline: every call makes
// exactly one attempt. Pair with IsRetryable to orchestrate custom loops.
func WithRetryDisabled() Option {
	return func(c *Client) { c.retryOff = true }
}

// WithLimiter installs a proactive rate limiter: Wait runs before every
// attempt, including retries.
func WithLimiter(l Limiter) Option {
	return func(c *Client) {
		if l == nil {
			c.storeConfigErr("WithLimiter", "limiter must not be nil")
			return
		}
		c.limiter = l
	}
}

// WithLogger enables slog.DebugContext tracing of requests and responses.
// The Authorization header is always redacted; errors are logged or
// returned, never both.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) {
		if l == nil {
			c.storeConfigErr("WithLogger", "logger must not be nil")
			return
		}
		c.logger = l
	}
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
