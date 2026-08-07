package planner

import (
	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/parser"
	"github.com/Emmanuel-MacAnThony/current/internal/plan"
)

// Planner is the real adapter behind the transport's Planner port. It produces a
// subscription's operator plus the tables its query reads. If the query compiles
// to a rule we can evaluate in memory, the operator is an in-memory Filter;
// otherwise it's a ReEval (the safe re-run path). A nil operator only comes back
// with a non-nil error (the SQL didn't parse) — the caller rejects the subscribe
// on that error before the operator is ever used.
type Planner struct{}

func (Planner) Plan(sql, key string) (domain.Operator, []string, error) {
	tables, err := parser.Parser{}.Tables(sql)
	if err != nil {
		return nil, nil, err
	}
	// (Tables and Compile each parse the SQL once — a double parse, but subscribe
	// is rare, so we keep the two concerns separate rather than optimize it away.)
	if rule, ok := Compile(sql); ok {
		return plan.Filter{Rule: rule, Key: key}, tables, nil
	}
	return plan.ReEval{}, tables, nil
}
