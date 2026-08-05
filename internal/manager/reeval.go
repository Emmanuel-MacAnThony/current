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
