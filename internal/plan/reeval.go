package plan

import "github.com/Emmanuel-MacAnThony/current/internal/domain"

// ReEval is the catch-all operator: it can't compute a delta in memory, so it
// always signals "re-eval me" (ok=false). The engine then re-runs the query and
// diffs by key — the safe path for every query we couldn't compile to a Filter
// (joins, aggregates, functions we don't handle). The re-run itself lives in the
// engine, not here, because it needs to happen OUTSIDE the manager lock; this
// operator only declares that it needs it. Holds no state.
type ReEval struct{}

func (ReEval) Apply(event domain.ChangeEvent, current domain.ResultSet) (domain.Delta, bool) {
	return domain.Delta{}, false
}
