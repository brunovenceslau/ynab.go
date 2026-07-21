// Command delta-sync keeps a local transaction store in sync with a plan
// using delta reads, printing what changed on each pass.
//
//	YNAB_TOKEN=... go run ./examples/delta-sync
//
// The point of delta sync: after the first full read, each pass fetches
// only what changed since the stored cursor — new/edited rows, plus
// tombstones for deletions — so a polling integration stays well inside
// YNAB's ~200 requests/hour quota. Persist the SyncState (it is plain
// JSON) and the next process resumes exactly where this one left off.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"pkg.venceslau.dev/ynab"
)

// statePath holds only cursors and a concrete plan id — no credentials —
// so it is safe to keep in the working directory (and to gitignore).
const statePath = "synced-state.json"

func main() {
	if err := run(); err != nil {
		slog.Error("delta sync", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// A long-running poller shuts down cleanly on Ctrl-C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := ynab.New(os.Getenv("YNAB_TOKEN"))

	// Persisted sync state MUST key to a concrete plan id, never the
	// last-used/default alias: a stored cursor is only valid for the plan
	// it came from, and the alias would hide a plan switch from Delta's
	// guard, silently corrupting the store. So resolve a real id first.
	plans, err := client.Plans(ctx)
	if err != nil {
		return err
	}
	if len(plans.Plans) == 0 {
		return errors.New("no plans reachable by this token")
	}
	plan := client.Plan(ynab.PlanID(plans.Plans[0].ID))

	// The store is your local cache: id -> transaction, carried across
	// passes. MergeByID upserts changed rows and drops tombstoned ones.
	store := map[string]ynab.TransactionBase{}
	st := loadState() // resume the cursor from disk if a prior run saved one

	for pass := 1; ; pass++ {
		detail, err := plan.Delta(ctx, st)
		if err != nil {
			return err
		}
		before := len(store)
		store = ynab.MergeByID(store, detail.Transactions)
		fmt.Printf("pass %d: %d changed rows, store now holds %d (was %d)\n",
			pass, len(detail.Transactions), len(store), before)

		if err := saveState(st); err != nil { // persist the advanced cursor
			return err
		}
		select {
		case <-ctx.Done():
			fmt.Println("shutting down; cursor saved")
			return nil
		case <-time.After(30 * time.Second):
		}
	}
}

func loadState() *ynab.SyncState {
	st := &ynab.SyncState{}
	if raw, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(raw, st)
	}
	return st
}

// saveState writes the cursor atomically: a crash mid-write must not
// truncate the file to a partial cursor.
func saveState(st *ynab.SyncState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := statePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, statePath)
}
