package api

import (
	"context"
	"sync"

	"github.com/coder/websocket"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
)

// wsConn adapts a coder/websocket connection to the standardized
// domain.Connection interface, so the manager and dispatch only ever see "a
// thing I can Send to and Close" — never the websocket library. It also owns the
// per-connection cancel, so Close both unblocks the read loop and shuts the
// socket — the hook that lets outside code (shutdown, per-client disconnect) tear
// this connection down.
type wsConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	c      *websocket.Conn
	once   sync.Once
}

// Compile-time proof that the adapter satisfies the standard interface.
var _ domain.Connection = (*wsConn)(nil)

func (w *wsConn) Send(msg []byte) error {
	return w.c.Write(w.ctx, websocket.MessageText, msg)
}

// Close cancels the read loop and closes the socket. Idempotent — it can be
// called by the handler's defer, a per-client Disconnect, and shutdown, in any
// order, without double-closing.
func (w *wsConn) Close() error {
	w.once.Do(func() {
		w.cancel()
		_ = w.c.CloseNow()
	})
	return nil
}
