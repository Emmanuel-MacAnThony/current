package manager

import (
	"sync"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/pkg/result"
)

// Manager owns the live, in-memory set of connected clients and their
// subscriptions, and the single lock guarding the whole nested structure.
// Nothing here is persisted — it rebuilds as clients reconnect.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*domain.Client // clientID -> Client
}

func New() *Manager {
	return &Manager{clients: make(map[string]*domain.Client)}
}

// Register records a freshly connected client (done on handshake, before any
// subscribe). It returns the stored client, plus ok=false if the id is already
// taken — an astronomically-unlikely clash the manager refuses to overwrite, so
// the caller retries with a new id. Uniqueness lives here because the manager
// owns the map; generating the id is someone else's job (utils.NewID).
func (m *Manager) Register(clientID string, conn domain.Connection) (*domain.Client, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.clients[clientID]; exists {
		return nil, false
	}
	c := domain.NewClient(clientID, conn)
	m.clients[clientID] = c
	return c, true
}

// Unregister drops a client and, with it, every subscription it owned — the
// cascade that makes "state dies with the connection" a single delete. It's
// system-triggered teardown (the transport's defer on disconnect), not a client
// request, so it's idempotent and infallible: unregistering an unknown or
// already-gone client is a harmless no-op.
func (m *Manager) Unregister(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, clientID)
}

// Disconnect is the external control point to force a client off (e.g. a future
// dead-client heartbeat, or an admin action). It only closes the client's
// connection — via the standardized Close, so the manager stays ignorant of
// websockets — which unblocks that client's read loop; the loop's deferred
// Unregister then removes it. Idempotent: an unknown client is a no-op. We grab
// the conn under the lock, then close outside it (never hold the lock across I/O).
func (m *Manager) Disconnect(clientID string) {
	m.mu.Lock()
	client, ok := m.clients[clientID]
	m.mu.Unlock()
	if !ok {
		return
	}
	_ = client.Conn.Close()
}

// Subscribe registers one subscription under a client. The manager is the
// authority on existence: it resolves the client from its own map rather than
// trusting the caller, so subscribing before the connection is registered fails
// loudly. Validation failures change nothing.
func (m *Manager) Subscribe(in SubscribeInput) result.Result[SubscribeOutput] {
	// Pure input checks first — no lock needed to look at the input.
	if in.ID == "" {
		return result.Fail[SubscribeOutput](ErrEmptySubID)
	}
	if in.SQL == "" {
		return result.Fail[SubscribeOutput](ErrEmptySQL)
	}

	// Everything below reads/writes the shared client state, so hold the lock.
	m.mu.Lock()
	defer m.mu.Unlock()

	// Contract: you must be registered (which the handshake does) before you can
	// subscribe. The manager checks its own map — it doesn't take the caller's word.
	client, ok := m.clients[in.ClientID]
	if !ok {
		return result.Fail[SubscribeOutput](ErrClientNotFound)
	}

	// A reused id is a client bug, not a re-subscribe: reject and leave the
	// existing subscription untouched rather than silently overwriting it.
	if _, exists := client.Subs[in.ID]; exists {
		return result.Fail[SubscribeOutput](ErrDuplicateSub)
	}

	client.Subs[in.ID] = &domain.Subscription{
		ID:       in.ID,
		ClientID: in.ClientID,
		SQL:      in.SQL,
		Result:   in.Result, // initial rows (computed at the edge) become this sub's Memory
	}
	return result.Ok(SubscribeOutput{})
}

// Unsubscribe removes one subscription from a client. Strict, mirroring
// Subscribe: an empty id, an unregistered client, or a subscription that isn't
// there are all rejected (they signal a frontend bug), and nothing is removed on
// any failure.
func (m *Manager) Unsubscribe(in UnsubscribeInput) result.Result[UnsubscribeOutput] {
	if in.ID == "" {
		return result.Fail[UnsubscribeOutput](ErrEmptySubID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[in.ClientID]
	if !ok {
		return result.Fail[UnsubscribeOutput](ErrClientNotFound)
	}
	if _, exists := client.Subs[in.ID]; !exists {
		return result.Fail[UnsubscribeOutput](ErrSubNotFound)
	}

	delete(client.Subs, in.ID)
	return result.Ok(UnsubscribeOutput{})
}
