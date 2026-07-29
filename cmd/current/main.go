// Command current is the composition root: build the pieces, wire them, run, and
// shut down cleanly. Only this file knows how the parts fit together.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Emmanuel-MacAnThony/current/internal/api"
	"github.com/Emmanuel-MacAnThony/current/internal/manager"
	"github.com/Emmanuel-MacAnThony/current/internal/postgres"
	"github.com/Emmanuel-MacAnThony/current/internal/transport"
)

func main() {
	// A context cancelled on Ctrl-C / kill. As the server's BaseContext, every
	// connection derives from it — cancelling it unblocks every read loop.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The one database the engine watches — configured here, never by clients.
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://ledger:ledger@localhost:5432/ledger"
	}
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		log.Fatalf("current: cannot open db pool: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("current: cannot reach db at %s: %v", dsn, err)
	}

	m := manager.New()
	runner := postgres.NewQueryRunner(pool)
	dispatcher := transport.NewServer(m, runner)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", api.WSHandler(m, dispatcher))

	srv := &http.Server{
		Addr:        ":8080",
		Handler:     mux,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	go func() {
		log.Printf("current: listening on %s (watching %s)", srv.Addr, dsn)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("current: server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("current: shutting down...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("current: graceful shutdown timed out: %v", err)
	}
	log.Println("current: stopped")
}
