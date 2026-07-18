package ynab

import (
	"net/url"
	"strconv"
)

// ServerKnowledge is the API's opaque, monotonically increasing delta
// cursor, scoped per (plan, stream). Pass it back with Since to receive
// only entities changed after it.
type ServerKnowledge int64

// ListOption tunes a delta-capable list call. Since is the only spelling.
type ListOption struct {
	apply func(url.Values)
}

// Since requests only entities changed after cursor k — the
// last_knowledge_of_server delta mechanism. A zero k sends the parameter
// with value 0, which the server treats as a full read; omit the option
// for a plain full read.
func Since(k ServerKnowledge) ListOption {
	return ListOption{apply: func(q url.Values) {
		q.Set("last_knowledge_of_server", strconv.FormatInt(int64(k), 10))
	}}
}

// applyListOptions folds opts into q, allocating only when needed.
func applyListOptions(q url.Values, opts []ListOption) url.Values {
	for _, opt := range opts {
		if opt.apply == nil {
			continue
		}
		if q == nil {
			q = url.Values{}
		}
		opt.apply(q)
	}
	return q
}

// Syncable is implemented by every synced entity: SyncID keys the entity
// in a local store, IsDeleted marks delta tombstones. (Entities keep the
// wire field Deleted bool; the method name avoids the field/method
// collision.)
type Syncable interface {
	SyncID() string
	IsDeleted() bool
}

// MergeByID folds a delta batch into a local store: tombstones delete,
// everything else upserts. It returns the map, allocating one when local
// is nil (the first sync). The map is caller-synchronized — guard it
// yourself if goroutines share it. Prefer pointer element types for wide
// entities (map[string]*Transaction) to avoid copying on every upsert.
func MergeByID[T Syncable](local map[string]T, changes []T) map[string]T {
	if local == nil {
		local = make(map[string]T, len(changes))
	}
	for _, change := range changes {
		if change.IsDeleted() {
			delete(local, change.SyncID())
			continue
		}
		local[change.SyncID()] = change
	}
	return local
}
