package manager

import (
	"github.com/Emmanuel-MacAnThony/current/internal/diff"
	"github.com/Emmanuel-MacAnThony/current/internal/domain"
)

// SubRef is a flat, lock-free copy of what the engine needs to re-run one
// subscription: which client owns it, its id, and its SQL. SubsForTable hands
// these out so the engine can run the (slow) queries with no lock held.
type SubRef struct {
	ClientID string
	SubID    string
	SQL      string
}

// ApplyResult is the second bracket op: the atomic diff-and-swap. Under the
// write lock it diffs the freshly-computed result against the sub's stored
// memory, overwrites memory with the fresh result, and returns the delta plus
// the client's connection so the engine can push. Doing the diff and the swap
// together under one lock keeps memory and the delta consistent.
//
// ok=false means the sub vanished while its query was running (the client
// disconnected, or unsubscribed) — the engine simply drops the result.
func (m *Manager) ApplyResult(clientID, subID string, next domain.ResultSet) (domain.Delta, domain.Connection, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[clientID]
	if !ok {
		return domain.Delta{}, nil, false
	}
	sub, ok := client.Subs[subID]
	if !ok {
		return domain.Delta{}, nil, false
	}

	delta := diff.ByKey(sub.Result, next, sub.Key)
	sub.Result = next // memory is now what the client will have seen after this push
	return delta, client.Conn, true
}

// ApplyEvent runs the sub's operator against one change, under the lock. If the
// operator handled it in memory (a Filter), the delta is folded into the sub's
// memory and returned with needReeval=false. If the operator deferred (a ReEval,
// or no operator), needReeval=true is returned so the engine re-runs the query
// OUTSIDE the lock. ok=false means the sub vanished mid-flight — skip it.
//
// The operator's in-memory work is fast, so it's safe to run here under the lock;
// the slow re-run is exactly what we push back out to the engine.
func (m *Manager) ApplyEvent(clientID, subID string, event domain.ChangeEvent) (delta domain.Delta, conn domain.Connection, needReeval bool, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[clientID]
	if !exists {
		return domain.Delta{}, nil, false, false
	}
	sub, exists := client.Subs[subID]
	if !exists {
		return domain.Delta{}, nil, false, false
	}

	// No operator (shouldn't happen — the planner always sets one) → re-eval, safe.
	if sub.Operator == nil {
		return domain.Delta{}, client.Conn, true, true
	}

	delta, handled := sub.Operator.Apply(event, sub.Result)
	if !handled {
		return domain.Delta{}, client.Conn, true, true // operator deferred → re-eval
	}

	sub.Result = applyDelta(sub.Result, delta, sub.Key) // fold the change into memory
	return delta, client.Conn, false, true
}

// applyDelta folds a delta into a result set by key: drop removed rows, swap in
// modified ones, append added ones. This is the in-memory update the re-eval path
// gets "for free" from a full re-run — here we do it directly from the delta.
func applyDelta(rows domain.ResultSet, d domain.Delta, key string) domain.ResultSet {
	if len(d.Removed) > 0 || len(d.Modified) > 0 {
		removed := make(map[any]bool, len(d.Removed))
		for _, r := range d.Removed {
			removed[r[key]] = true
		}
		modified := make(map[any]domain.Row, len(d.Modified))
		for _, r := range d.Modified {
			modified[r[key]] = r
		}

		kept := make(domain.ResultSet, 0, len(rows))
		for _, r := range rows {
			k := r[key]
			if removed[k] {
				continue
			}
			if m, ok := modified[k]; ok {
				kept = append(kept, m)
				continue
			}
			kept = append(kept, r)
		}
		rows = kept
	}
	return append(rows, d.Added...)
}
