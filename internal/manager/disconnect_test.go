package manager_test

import (
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/manager"
)

// recordConn notes whether Close was called, so we can assert Disconnect reaches
// the socket. (The actual client removal happens later, via the read loop's
// defer Unregister once Close unblocks it — not the manager's job here.)
type recordConn struct{ closed bool }

func (c *recordConn) Send([]byte) error { return nil }
func (c *recordConn) Close() error      { c.closed = true; return nil }

func TestDisconnect_ClosesTheClientsConnection(t *testing.T) {
	m := manager.New()
	conn := &recordConn{}
	m.Register("c1", conn)

	m.Disconnect("c1")

	if !conn.closed {
		t.Fatalf("expected the client's connection to be closed")
	}
}

func TestDisconnect_UnknownClient_IsNoOp(t *testing.T) {
	m := manager.New()

	m.Disconnect("ghost") // must not panic; nothing to close
}
