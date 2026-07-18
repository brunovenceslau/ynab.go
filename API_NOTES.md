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
- **Status:** worked-around

## delta endpoints — docs say 9, spec shows 11

- **Date:** 2026-07-18
- **Docs say:** the api.ynab.com prose on delta requests lists 9 endpoints
  supporting `last_knowledge_of_server`.
- **Reality shows:** OpenAPI 1.86.0 declares the `last_knowledge_of_server`
  query parameter on 11 operations (verified by grep over the vendored spec).
- **Impact:** the spec is authoritative — the library exposes `Since` on all
  11 spec-declared delta operations; the G1 table encodes the same 11.
- **Status:** worked-around
