# split-transaction

Creates a split transaction whose legs sum exactly to the parent via
`SplitEven`. **Writes** — use a scratch plan.

```sh
YNAB_TOKEN=... YNAB_PLAN_ID=... YNAB_ACCOUNT_ID=... go run ./examples/split-transaction
```
