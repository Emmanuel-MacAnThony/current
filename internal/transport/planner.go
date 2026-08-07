package transport

import "github.com/Emmanuel-MacAnThony/current/internal/domain"

// Planner turns a subscription's SQL into the two things the subscribe path needs:
// the operator that will react to changes (a Filter if we could compile the query,
// else a ReEval), and the tables the query reads (for the matchmaker index). Owned
// here (consumer-defined); the planner package satisfies it structurally, so
// transport never imports the planner.
type Planner interface {
	Plan(sql, key string) (op domain.Operator, tables []string, err error)
}
