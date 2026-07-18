package ynab

// Test-only exports: the root test package proves internals that have no
// public observation path yet.

// DecodeWireError exposes the injected transport error decoder to tests.
var DecodeWireError = decodeWireError
