package engine

import (
	"context"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/manager"
)

// QueryRunner re-runs a subscription's SQL to get its fresh result set. The
// engine owns this port (consumer-defined) so the Postgres adapter can satisfy
// it structurally without the engine importing infra.
type QueryRunner interface {
	Run(sql string) (domain.ResultSet, error)
}

// Pusher delivers a computed diff to a single subscriber. Owned here for the
// same reason: the transport adapter satisfies it structurally.
type Pusher interface {
	Push(conn domain.Connection, subID string, delta domain.Delta)
}

// Engine is the change-flow orchestrator. It sits between the change source
// (the WAL watcher) and the subscribers: on every change it re-evaluates the
// live queries, diffs each result against what the client last saw, and pushes
// the difference.
type Engine struct {
	source ChangeSource
	subs   *manager.Manager
	runner QueryRunner
	pusher Pusher
}

func New(source ChangeSource, subs *manager.Manager, runner QueryRunner, pusher Pusher) *Engine {
	return &Engine{source: source, subs: subs, runner: runner, pusher: pusher}
}

// Run wires onChange as the watcher's callback and blocks until the source
// stops (context cancelled or stream error). onChange is the seam: the watcher
// produces changes, the engine consumes them.
func (e *Engine) Run(ctx context.Context) error {
	return e.source.Run(ctx, e.onChange)
}

// onChange is invoked once per database change. The matchmaker routes it: only
// the subscriptions that read the changed table are re-evaluated, not every sub.
//
// The expensive query runs *outside* the manager's lock. SubsForTable and
// ApplyResult are the two fast lock ops that bracket it: SubsForTable reads the
// routed subs, we re-run each query in the open, then ApplyResult atomically
// diffs the fresh result against stored memory and swaps it in.
func (e *Engine) onChange(event domain.ChangeEvent) {
	for _, ref := range e.subs.SubsForTable(event.Table) {
		// First ask the sub's operator (under the manager lock). A Filter computes
		// the delta in memory and we're done; a ReEval defers and we fall through
		// to the slow re-run.
		delta, conn, needReeval, ok := e.subs.ApplyEvent(ref.ClientID, ref.SubID, event)
		if !ok {
			continue // sub vanished mid-flight
		}

		if needReeval {
			newResult, err := e.runner.Run(ref.SQL) // slow I/O, no lock held here
			if err != nil {
				// A single failed re-eval shouldn't sink the flow; the next change
				// retries it. TODO: surface via a logger.
				continue
			}
			delta, conn, ok = e.subs.ApplyResult(ref.ClientID, ref.SubID, newResult)
			if !ok {
				continue
			}
		}

		if !delta.IsEmpty() {
			e.pusher.Push(conn, ref.SubID, delta)
		}
	}
}
