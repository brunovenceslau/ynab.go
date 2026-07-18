// Package ynab is a complete Go client for the YNAB API v1: all 44
// operations behind a domain-first surface, exact money arithmetic in
// Milliunits, first-class delta sync, and zero runtime dependencies.
//
//	client := ynab.New(os.Getenv("YNAB_TOKEN"))
//	plan := client.Plan(ynab.PlanIDLastUsed)
//	accounts, knowledge, err := plan.Accounts.List(ctx)
//
// # Authentication
//
// New takes a personal access token (create one at app.ynab.com under
// Developer Settings). For OAuth integrations whose access tokens expire,
// NewWithTokenSource consults a TokenSource before every attempt, so a
// refreshed token is picked up mid-retry. The token travels only in the
// Authorization header and is always redacted from WithLogger tracing.
//
// Construction never fails: a bad option (an unparseable WithBaseURL, a
// nil limiter) is stored and every method returns it as *ArgumentError
// before any I/O — nothing silently falls back.
//
// # The plan handle
//
// A plan is what YNAB historically called a budget; the API spec now
// documents every route under /plans, and this package follows that
// vocabulary. Everything plan-scoped hangs off the handle Client.Plan
// returns — without I/O, so it is free to build wherever needed:
//
//	plan := client.Plan(ynab.PlanIDLastUsed)
//	groups, knowledge, err := plan.Categories.List(ctx)
//	category, knowledge, err := plan.Categories.Assign(ctx, ynab.CurrentMonth(), categoryID, budgeted)
//
// PlanIDLastUsed and PlanIDDefault are server-resolved literals besides
// concrete plan ids. The zero Client and the zero Plan are unusable by
// design: handles come only from constructors, so a bound plan id can
// never drift from its services. *Client, *Plan, and all services are
// safe for concurrent use; configuration is set once at construction.
//
// Write payloads use the Optional tri-state — unset (field omitted),
// SetNull (clear the value server-side), Set (send it, zero values
// included: Set(false) reaches the wire). Omitting a field and clearing
// it are different operations.
//
// # Delta sync
//
// YNAB's quota is roughly 200 requests per hour — delta sync is how a
// polling integration lives inside it. Delta-capable reads return a
// ServerKnowledge cursor. Hand it back with
// Since (or TransactionFilter.Since) to receive only what changed —
// deletions arriving as tombstones that MergeByID folds into a local
// store. SyncState is the JSON-persistable bundle of cursors, and
// Plan.Delta is the one-request full-plan variant:
//
//	st := &ynab.SyncState{}      // persist between runs
//	detail, err := plan.Delta(ctx, st)
//
// Cursor spaces are per (plan, stream): never reuse one plan's cursor on
// another (Plan.Delta rejects it), and never advance a cursor from the
// zero ServerKnowledge the hybrid transaction lists may return. Wire
// asymmetries are surfaced, not papered over: Accounts.Create returns no
// cursor while Categories.Create does, money movements return one but
// accept none, and getCategories deltas nest changed categories inside
// their groups — flatten before merging (see the CategoriesService.List
// example). MergeByID maps and SyncState are caller-synchronized; prefer
// pointer element types for wide entities (map[string]*ynab.Transaction).
//
// # Errors
//
// API failures are *Error values matched with errors.Is against the
// sentinel taxonomy. Matching is exact AND class-wide: a 403.1 response
// satisfies both ErrSubscriptionLapsed and ErrForbidden. Client-side
// pre-flight failures — spec-stated invariants only, like a zero Month,
// declared length bounds, or split legs that do not sum — are
// *ArgumentError, and the request is never sent. Error.RetryAfter of
// zero means unknown, never "retry immediately".
//
// The built-in pipeline already retries 429 (any verb) and 500/503
// (GET/DELETE unless RetryWrites); IsRetryable exposes the same
// classification for custom loops. errors.Is(err, context.Canceled) and
// context.DeadlineExceeded work through the chain, as does errors.As
// for net.Error — but the concrete transport error types in the chain
// are not part of the compatibility promise.
//
// One surgical divergence: a plan with no scheduled transactions answers
// 404 to the list itself, so ScheduledTransactionsService.List — and
// only that method — folds the 404 into an empty result. Every other
// operation's 404 is ErrNotFound (ErrRouteNotFound or
// ErrResourceNotFound by sub-code).
//
// # Testing your integration
//
// Two dependency-free seams exist for consumer tests: WithBaseURL
// pointed at an httptest.Server, and WithHTTPClient carrying a fake
// http.RoundTripper — see their examples. Return arities are fixed per
// operation (they mirror the wire, cursor included) and are not expected
// to change within v1.
package ynab
