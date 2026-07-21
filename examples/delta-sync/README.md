# delta-sync

Keeps a local transaction store in sync using delta reads, persisting the
cursor between runs. Shows why the `store` and `SyncState` matter: after
the first full read, each pass fetches only what changed (tombstones
included).

```sh
YNAB_TOKEN=... go run ./examples/delta-sync
```
