package api

import (
	"context"
	"net/http"

	"github.com/coder/websocket"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/manager"
	"github.com/Emmanuel-MacAnThony/current/internal/transport"
	"github.com/Emmanuel-MacAnThony/current/pkg/utils"
)

// WSHandler is the connection lifecycle: accept the socket, register the client
// (minting its id here — the client never supplies it), then read messages and
// hand each to the dispatcher until the socket drops. The read loop runs on a
// per-connection context derived from the request's — which itself derives from
// the server's BaseContext — so the loop unblocks on any of: client disconnect,
// server shutdown, or an explicit Close (per-client Disconnect). On exit the
// deferred Close + Unregister tear everything down.
func WSHandler(m *manager.Manager, d *transport.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		conn := &wsConn{ctx: ctx, cancel: cancel, c: c}
		defer conn.Close() // cancels ctx + closes socket, whatever the exit reason

		client := register(m, conn)
		defer m.Unregister(client.ID)

		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return // disconnect / shutdown / forced close → defers run
			}
			d.HandleMessage(client.ID, conn, data)
		}
	}
}

// register mints a fresh id and stores the client, retrying on the
// astronomically-unlikely id clash the manager rejects. Generation (utils) and
// uniqueness (manager) stay separate; this loop just marries them.
func register(m *manager.Manager, conn domain.Connection) *domain.Client {
	for {
		if client, ok := m.Register(utils.NewID(), conn); ok {
			return client
		}
	}
}
