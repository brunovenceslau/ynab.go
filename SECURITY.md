# Security policy

This library handles YNAB Personal Access Tokens — credentials to real
financial data. Treat every report accordingly.

## Supported versions

Only the latest v1.x release receives security fixes.

## Reporting a vulnerability

Do **not** open a public issue. Use GitHub's private vulnerability
reporting: [Report a vulnerability](https://github.com/brunovenceslau/ynab.go/security/advisories/new).

Reports are acknowledged within 7 days. Disclosure is coordinated: we
agree on a timeline together before anything becomes public.

## What counts

Anything that could expose a token (logs, errors, User-Agent, headers to
unexpected hosts), weaken TLS posture, or make the client emit requests
the caller did not ask for. The token-redaction promise is asserted by
the integration suite on real traffic — a redaction bypass is a
vulnerability, not a bug.
