package transport_test

import (
	"encoding/json"
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/manager"
	"github.com/Emmanuel-MacAnThony/current/internal/transport"
)

// captureConn is a fake Connection that records every frame the dispatch sends,
// so tests can assert what went back down the socket.
type captureConn struct{ sent [][]byte }

func (c *captureConn) Send(msg []byte) error { c.sent = append(c.sent, msg); return nil }
func (c *captureConn) Close() error          { return nil }

// frame is the test's view of an outbound message — decoupled from the
// transport's internal type.
type frame struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Message string `json:"message"`
}

func decode(t *testing.T, b []byte) frame {
	t.Helper()
	var f frame
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("sent frame is not valid json: %v", err)
	}
	return f
}

// setup returns a dispatch server, the registered client "c1" (so we can inspect
// its Subs), and the conn that captures frames — the same conn the client is
// registered with, as it would be in the real read loop.
func setup() (*transport.Server, *domain.Client, *captureConn) {
	m := manager.New()
	conn := &captureConn{}
	client, _ := m.Register("c1", conn)
	return transport.NewServer(m), client, conn
}

func TestDispatch_ValidSubscribe_RegistersAndAcks(t *testing.T) {
	s, client, conn := setup()

	s.HandleMessage("c1", conn, []byte(`{"type":"subscribe","id":"orders","sql":"SELECT 1"}`))

	if _, ok := client.Subs["orders"]; !ok {
		t.Fatalf("expected subscription 'orders' registered")
	}
	if len(conn.sent) != 1 {
		t.Fatalf("expected 1 frame sent, got %d", len(conn.sent))
	}
	if f := decode(t, conn.sent[0]); f.Type != "ack" || f.ID != "orders" {
		t.Fatalf("expected ack for 'orders', got %+v", f)
	}
}

func TestDispatch_SubscribeError_SendsErrorFrameAndRegistersNothing(t *testing.T) {
	s, client, conn := setup()

	s.HandleMessage("c1", conn, []byte(`{"type":"subscribe","id":"orders","sql":""}`))

	if len(client.Subs) != 0 {
		t.Fatalf("expected nothing registered, got %d", len(client.Subs))
	}
	if len(conn.sent) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(conn.sent))
	}
	f := decode(t, conn.sent[0])
	if f.Type != "error" || f.ID != "orders" || f.Message != manager.ErrEmptySQL.Error() {
		t.Fatalf("expected error frame with empty-sql message, got %+v", f)
	}
}

func TestDispatch_MalformedJSON_SendsErrorFrame(t *testing.T) {
	s, client, conn := setup()

	s.HandleMessage("c1", conn, []byte(`this is not json`))

	if len(client.Subs) != 0 {
		t.Fatalf("expected nothing registered")
	}
	if len(conn.sent) != 1 {
		t.Fatalf("expected 1 error frame, got %d", len(conn.sent))
	}
	if f := decode(t, conn.sent[0]); f.Type != "error" {
		t.Fatalf("expected error frame, got %+v", f)
	}
}

func TestDispatch_UnknownType_IsIgnored(t *testing.T) {
	s, client, conn := setup()

	s.HandleMessage("c1", conn, []byte(`{"type":"whoami","id":"x"}`))

	if len(client.Subs) != 0 {
		t.Fatalf("expected nothing registered")
	}
	if len(conn.sent) != 0 {
		t.Fatalf("expected no frame for unknown type, got %d", len(conn.sent))
	}
}

func TestDispatch_ValidUnsubscribe_RemovesAndAcks(t *testing.T) {
	s, client, conn := setup()
	s.HandleMessage("c1", conn, []byte(`{"type":"subscribe","id":"orders","sql":"SELECT 1"}`))
	conn.sent = nil // drop the subscribe ack; we only care about the unsubscribe

	s.HandleMessage("c1", conn, []byte(`{"type":"unsubscribe","id":"orders"}`))

	if _, ok := client.Subs["orders"]; ok {
		t.Fatalf("expected subscription removed")
	}
	if len(conn.sent) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(conn.sent))
	}
	if f := decode(t, conn.sent[0]); f.Type != "ack" || f.ID != "orders" {
		t.Fatalf("expected ack for 'orders', got %+v", f)
	}
}
