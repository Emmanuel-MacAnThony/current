package transport

import (
	"encoding/json"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/manager"
)

// inMessage is the wire shape a client sends. It is NOT the use-case Input: the
// handler assembles that, injecting clientID from the socket and the query result
// it computed — never from here.
type inMessage struct {
	Type string `json:"type"` // "subscribe" | "unsubscribe"
	ID   string `json:"id"`
	SQL  string `json:"sql"`
	Key  string `json:"key"` // column that identifies a row, for diffing; defaults to "id"
}

// outMessage is a frame we send back: initial data, an ack, a developer-facing
// error the client's console shows, or a live diff. A "data" frame carries the
// full Rows (the initial snapshot); a "diff" frame carries only what changed.
type outMessage struct {
	Type     string           `json:"type"` // "data" | "ack" | "error" | "diff"
	ID       string           `json:"id,omitempty"`
	Message  string           `json:"message,omitempty"`
	Rows     domain.ResultSet `json:"rows,omitempty"`
	Added    []domain.Row     `json:"added,omitempty"`
	Removed  []domain.Row     `json:"removed,omitempty"`
	Modified []domain.Row     `json:"modified,omitempty"`
}

// Server is the transport edge: it translates raw wire messages into manager
// calls and sends frames back. It holds the QueryRunner so it can run a
// subscription's query HERE, at the edge — outside the manager's lock — then hand
// the finished result to a fast Subscribe.
type Server struct {
	m      *manager.Manager
	runner QueryRunner
	parser SQLParser
}

func NewServer(m *manager.Manager, runner QueryRunner, parser SQLParser) *Server {
	return &Server{m: m, runner: runner, parser: parser}
}

// HandleMessage processes one inbound wire message. clientID is supplied by the
// caller (derived from the socket), never taken from the payload.
func (s *Server) HandleMessage(clientID string, conn domain.Connection, data []byte) {
	var msg inMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		send(conn, outMessage{Type: "error", Message: "malformed message"})
		return
	}

	switch msg.Type {
	case "subscribe":
		s.handleSubscribe(clientID, conn, msg)
	case "unsubscribe":
		res := s.m.Unsubscribe(manager.UnsubscribeInput{ClientID: clientID, ID: msg.ID})
		reply(conn, msg.ID, res.Err)
	default:
		// Unknown types are ignored — no frame.
	}
}

// handleSubscribe runs the query at the edge (outside the manager's lock), stores
// the result via a fast Subscribe, and sends the initial rows. Cheap input checks
// gate the query so we never hit the DB for a malformed request.
func (s *Server) handleSubscribe(clientID string, conn domain.Connection, msg inMessage) {
	if msg.ID == "" {
		send(conn, outMessage{Type: "error", ID: msg.ID, Message: manager.ErrEmptySubID.Error()})
		return
	}
	if msg.SQL == "" {
		send(conn, outMessage{Type: "error", ID: msg.ID, Message: manager.ErrEmptySQL.Error()})
		return
	}

	// Parse the tables the query reads (for the matchmaker index) before touching
	// the DB — an unparseable query is rejected here, with no round-trip. Without
	// these, the sub would be indexed under nothing and silently never update.
	tables, err := s.parser.Tables(msg.SQL)
	if err != nil {
		send(conn, outMessage{Type: "error", ID: msg.ID, Message: "invalid query: " + err.Error()})
		return
	}

	rows, err := s.runner.Run(msg.SQL) // slow I/O, but no manager lock is held here
	if err != nil {
		send(conn, outMessage{Type: "error", ID: msg.ID, Message: "query failed: " + err.Error()})
		return
	}

	key := msg.Key
	if key == "" {
		key = "id" // the common case: rows are identified by their id column
	}

	res := s.m.Subscribe(manager.SubscribeInput{ClientID: clientID, ID: msg.ID, SQL: msg.SQL, Key: key, Result: rows, Tables: tables})
	if res.Err != nil {
		send(conn, outMessage{Type: "error", ID: msg.ID, Message: res.Err.Error()})
		return
	}
	send(conn, outMessage{Type: "data", ID: msg.ID, Rows: rows})
}

// reply acks on success, or sends a developer-facing error frame on failure.
func reply(conn domain.Connection, id string, err error) {
	if err != nil {
		send(conn, outMessage{Type: "error", ID: id, Message: err.Error()})
		return
	}
	send(conn, outMessage{Type: "ack", ID: id})
}

// send marshals and writes a frame. A send failure means the socket is going
// away; the read loop will notice and unregister, so there's nothing useful to
// do with the error here.
func send(conn domain.Connection, msg outMessage) {
	b, _ := json.Marshal(msg)
	_ = conn.Send(b)
}
