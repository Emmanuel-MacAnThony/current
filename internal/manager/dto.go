package manager

import "github.com/Emmanuel-MacAnThony/current/internal/domain"

// SubscribeInput is the use-case contract for a subscribe — NOT the wire message.
// The browser sends only {id, sql} and never its ClientID; the handler derives
// ClientID from the socket, runs the query at the edge, and assembles this Input.
// So the Input carries everything the use case needs, put together at the edge —
// including the already-computed initial Result (the query is run outside the
// manager's lock, never inside it).
type SubscribeInput struct {
	ClientID string
	ID       string // subID, client-chosen
	SQL      string
	Key      string           // column that identifies a result row, for diffing (default "id")
	Result   domain.ResultSet // initial rows, computed by the caller before this call
	Tables   []string         // tables the query reads, parsed by the handler — the matchmaker index keys
	Operator domain.Operator  // how this sub reacts to a change (Filter / ReEval), chosen by the handler
}

// SubscribeOutput is empty for now: success is simply "no error." It grows to
// carry the initial rows once the query-run slice lands.
type SubscribeOutput struct{}

// UnsubscribeInput mirrors that shape: ClientID (from the socket) + the subID.
type UnsubscribeInput struct {
	ClientID string
	ID       string // subID to remove
}

type UnsubscribeOutput struct{}
