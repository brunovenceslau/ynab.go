# API notes

Observed-vs-documented divergences and undocumented behaviors of the YNAB API.
Every workaround in this library points at an entry here; entries are appended
at the moment of discovery and never silently removed. The vendored
`openapi.yaml` and https://api.ynab.com are the primary sources; where a
secondary source (research notes, prior-generation code) disagrees with them,
the entry records the disagreement and the primary source wins.

Entry template:

```
## <operationId or area> — <one-line title>

- **Date:** YYYY-MM-DD
- **Docs say:** <claim> (source + spec version)
- **Reality shows:** <observed behavior> (evidence)
- **Impact:** <what this library does about it>
- **Status:** open | worked-around | resolved
```

## paths — `/budgets` is a legacy alias of `/plans`

- **Date:** 2026-07-18
- **Docs say:** OpenAPI 1.86.0 documents every collection under `/plans/...`;
  `/budgets` appears nowhere in the spec. The v1.79.0 API changelog states the
  old `/budgets` routes "continue to work" but are "no longer documented".
- **Reality shows:** the live API still serves the `/budgets/...` spellings as
  undocumented aliases of `/plans/...`.
- **Impact:** this library emits only the documented `/plans/...` paths; the
  alias is never generated and never accepted in helpers.
- **Status:** open

## getScheduledTransactions — empty plan answers 404, not an empty list

- **Date:** 2026-07-18
- **Docs say:** OpenAPI 1.86.0 declares a 200 `ScheduledTransactionsResponse`
  for the list and a 404 `ErrorResponse` for "not found".
- **Reality shows:** research notes report that a plan which has *never*
  had scheduled transactions answers 404.2 to the *list* itself. Live
  probe 2026-07-18 (API 1.86.0): a plan that previously had scheduled
  transactions and currently has none answers 200 with an empty list —
  so the 404 is specific to the never-had case, which is no longer
  reproducible on the dedicated test plan (and the API cannot create
  fresh plans). The fold stays as defense-in-depth.
- **Impact:** `Scheduled.List` — and only that method — folds the 404 into
  `([], 0, nil)`; every other operation's 404 stays `ErrResourceNotFound`.
  Both sides of the contrast are pinned by tests.
- **Status:** worked-around

## delta endpoints — docs say 9, spec shows 11

- **Date:** 2026-07-18
- **Docs say:** the api.ynab.com prose on delta requests lists 9 endpoints
  supporting `last_knowledge_of_server`.
- **Reality shows:** OpenAPI 1.86.0 declares the `last_knowledge_of_server`
  query parameter on 11 operations (verified by grep over the vendored spec).
- **Impact:** the spec is authoritative — the library exposes `Since` on all
  11 spec-declared delta operations, and its contract tests encode the same 11.
- **Status:** open

## response headers — no rate-limit header on live traffic

- **Date:** 2026-07-20
- **Docs say:** the api.ynab.com prose documents an `X-Rate-Limit` response
  header carrying the hourly quota state (e.g. `36/200`); the vendored
  OpenAPI 1.86.0 declares no response headers at all (zero `headers:` keys).
- **Reality shows:** live probe 2026-07-20 (GET `/v1/user`, personal access
  token): the full response-header set is `content-type`, `cache-control`,
  `x-request-id`, `x-runtime`, `heroku-*` routing headers, and security
  headers — nothing quota-shaped. Rate limiting is enforced (429s are real)
  but not surfaced through any response header.
- **Impact:** the live integration suite asserts no rate-limit header.
  Instead its transport records the union of response-header keys (plus
  values for any key matching `rate|limit|quota`) and logs them at suite
  end, so a renamed or returning header would be spotted in one run.
- **Status:** open

## plan_id `default` — resolves only for OAuth grants, 404.2 under a PAT

- **Date:** 2026-07-20
- **Docs say:** OpenAPI 1.86.0 describes `"default"` as resolving to the
  default plan when the OAuth authorization selected one, "equivalent to
  last-used otherwise".
- **Reality shows:** live probe 2026-07-20 (GET `/v1/plans/default/settings`,
  personal access token): the server answers 404.2 `resource_not_found` —
  under a PAT the sentinel does not fall back to last-used.
- **Impact:** documented on the `PlanIDDefault` constant. The live suite
  does not exercise the sentinel: under its PAT no assertion could predict
  a resolution, and the observed behavior is captured here instead.
- **Status:** open
