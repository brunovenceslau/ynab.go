# Examples

Runnable programs — most call the **real** YNAB API — unlike the godoc
[package examples](https://pkg.go.dev/pkg.venceslau.dev/ynab#pkg-examples)
(which use `httptest` so they run offline and prove behavior). Copy one,
export a token, and go.

| Program | What it shows | Needs |
|---|---|---|
| [`quickstart`](quickstart) | List your accounts | `YNAB_TOKEN` |
| [`delta-sync`](delta-sync) | Keep a local store in sync with delta reads | `YNAB_TOKEN` |
| [`split-transaction`](split-transaction) | Create a split via `SplitEven` (writes!) | `YNAB_TOKEN`, `YNAB_PLAN_ID`, `YNAB_ACCOUNT_ID` |
| [`testing-with-mock`](testing-with-mock) | Point the client at your own server for tests | — (offline) |

```sh
YNAB_TOKEN=your-personal-access-token go run ./examples/quickstart
```

Get a Personal Access Token at app.ynab.com → Settings → Developer
Settings. The write example creates and leaves a real transaction —
point it at a scratch plan.
