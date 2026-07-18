package ynab

// Test-only exports: the root test package proves internals that have no
// public observation path yet.

import "net/url"

// DecodeWireError exposes the injected transport error decoder to tests.
var DecodeWireError = decodeWireError

// ApplyListOptions exposes ListOption folding so tests can assert the
// encoded query parameters.
func ApplyListOptions(q url.Values, opts ...ListOption) url.Values {
	return applyListOptions(q, opts)
}

// BaseURLOf exposes the configured base URL so the write-contract harness
// can drive raw requests through the same server a client points at.
func BaseURLOf(c *Client) string {
	return c.baseURL.String()
}
