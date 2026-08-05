package transport

import "github.com/Emmanuel-MacAnThony/current/internal/domain"

// Messenger is the engine's Pusher adapter: it turns a computed Delta into a
// "diff" wire frame and writes it to one subscriber's connection. It's stateless
// (no fields) — everything it needs arrives in Push — so a zero value is usable.
// The engine depends on its own Pusher port; Messenger satisfies it structurally,
// so transport never imports engine.
type Messenger struct{}

// Push sends the delta down the client's socket, tagged with the subscription id
// so the client can route it to the right query. A send failure is swallowed for
// the same reason as elsewhere: the read loop will notice the dead socket and
// unregister — there's nothing useful to do with the error here.
func (Messenger) Push(conn domain.Connection, subID string, delta domain.Delta) {
	send(conn, outMessage{
		Type:     "diff",
		ID:       subID,
		Added:    delta.Added,
		Removed:  delta.Removed,
		Modified: delta.Modified,
	})
}
