package engine

import (
	"context"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
)

// ChangeSource is a stream of committed row changes the engine consumes. Run
// blocks, calling handle for each change until ctx is cancelled (clean stop) or
// the stream fails (returns the error). The Watcher — a Postgres
// logical-replication adapter — implements it; the engine depends only on this
// port, never on pglogrepl.
type ChangeSource interface {
	Run(ctx context.Context, handle func(domain.ChangeEvent)) error
}
