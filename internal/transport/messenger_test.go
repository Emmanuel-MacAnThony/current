package transport_test

import (
	"encoding/json"
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/transport"
)

// capturingConn records the last frame written, so we can inspect the wire shape.
type capturingConn struct{ sent [][]byte }

func (c *capturingConn) Send(b []byte) error { c.sent = append(c.sent, b); return nil }
func (c *capturingConn) Close() error        { return nil }

func TestMessenger_Push_SendsDiffFrame(t *testing.T) {
	conn := &capturingConn{}
	delta := domain.Delta{
		Added:   []domain.Row{{"id": 3}},
		Removed: []domain.Row{{"id": 2}},
	}

	transport.Messenger{}.Push(conn, "orders", delta)

	if len(conn.sent) != 1 {
		t.Fatalf("expected 1 frame sent, got %d", len(conn.sent))
	}

	var got struct {
		Type     string       `json:"type"`
		ID       string       `json:"id"`
		Added    []domain.Row `json:"added"`
		Removed  []domain.Row `json:"removed"`
		Modified []domain.Row `json:"modified"`
	}
	if err := json.Unmarshal(conn.sent[0], &got); err != nil {
		t.Fatalf("frame is not valid json: %v", err)
	}
	if got.Type != "diff" || got.ID != "orders" {
		t.Fatalf("wrong envelope: type=%q id=%q", got.Type, got.ID)
	}
	if len(got.Added) != 1 || got.Added[0]["id"] != float64(3) {
		t.Fatalf("expected id 3 added, got %+v", got.Added)
	}
	if len(got.Removed) != 1 || got.Removed[0]["id"] != float64(2) {
		t.Fatalf("expected id 2 removed, got %+v", got.Removed)
	}
	if len(got.Modified) != 0 {
		t.Fatalf("expected no modified, got %+v", got.Modified)
	}
}
