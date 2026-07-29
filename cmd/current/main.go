// Command current is the composition root: build the pieces, wire them, run, and
// shut down cleanly. Only this file knows how the parts fit together.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Emmanuel-MacAnThony/current/internal/api"
	"github.com/Emmanuel-MacAnThony/current/internal/manager"
	"github.com/Emmanuel-MacAnThony/current/internal/transport"
)

func main() {
	// A context cancelled on Ctrl-C / kill. Wiring it in as the server's
	// BaseContext means every connection's context derives from it, so cancelling
	// it unblocks every websocket read loop — the outside control over all conns.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	m := manager.New()
	dispatcher := transport.NewServer(m)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", api.WSHandler(m, dispatcher))

	srv := &http.Server{
		Addr:        ":8080",
		Handler:     mux,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	go func() {
		log.Printf("current: listening on %s (websocket at /ws)", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("current: server error: %v", err)
		}
	}()

	<-ctx.Done() // wait for a shutdown signal
	log.Println("current: shutting down...")

	// ctx is already cancelled, so the read loops are unblocking; give the server
	// a bounded window to drain in-flight handlers, then stop.
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("current: graceful shutdown timed out: %v", err)
	}
	log.Println("current: stopped")
}
