package transport

import (
	"encoding/json"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/manager"
)

// inMessage is the wire shape a client sends. It is NOT the use-case Input: the
// handler assembles that, injecting clientID from the socket — never from here.
type inMessage struct {
	Type string `json:"type"` // "subscribe" | "unsubscribe"
	ID   string `json:"id"`
	SQL  string `json:"sql"`
}

// outMessage is a frame we send back: an ack on success, or a developer-facing
// error the client's console shows.
type outMessage struct {
	Type    string `json:"type"` // "ack" | "error"
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
}

// Server is the transport edge: it translates raw wire messages into manager
// calls and sends ack/error frames back, so the manager never has to know about
// sockets or JSON.
type Server struct {
	m *manager.Manager
}

func NewServer(m *manager.Manager) *Server {
	return &Server{m: m}
}

// HandleMessage processes one inbound wire message. clientID is supplied by the
// caller (derived from the socket), never taken from the payload — that's what
// makes a client unable to act as anyone else.
func (s *Server) HandleMessage(clientID string, conn domain.Connection, data []byte) {
	var msg inMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		// A message we can't even parse is a developer error — tell them.
		send(conn, outMessage{Type: "error", Message: "malformed message"})
		return
	}

	switch msg.Type {
	case "subscribe":
		res := s.m.Subscribe(manager.SubscribeInput{ClientID: clientID, ID: msg.ID, SQL: msg.SQL})
		reply(conn, msg.ID, res.Err)
	case "unsubscribe":
		res := s.m.Unsubscribe(manager.UnsubscribeInput{ClientID: clientID, ID: msg.ID})
		reply(conn, msg.ID, res.Err)
	default:
		// Unknown types are ignored — no frame.
	}
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
