package plan

import (
	"fmt"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
)

// Filter is the incremental operator for a simple `SELECT … WHERE <rule>`: it
// turns one change into a delta in memory, never touching the database. Rule is
// the compiled WHERE; Key is the row-identity column (for matching the saved
// list). It always handles the event — a filter never needs to re-eval.
type Filter struct {
	Rule Rule
	Key  string
}

func (f Filter) Apply(event domain.ChangeEvent, current domain.ResultSet) (domain.Delta, bool) {
	switch event.Op {
	case domain.OpInsert:
		// A brand-new row joins the result only if it matches the filter.
		if f.Rule.Matches(event.New) {
			return domain.Delta{Added: []domain.Row{event.New}}, true
		}

	case domain.OpDelete:
		// The event carries only the key; "was it in the result?" is answered by
		// the saved list, and the removed row we report is the stored (full) one.
		if old, ok := f.find(current, event.Old); ok {
			return domain.Delta{Removed: []domain.Row{old}}, true
		}

	case domain.OpUpdate:
		old, wasIn := f.find(current, event.New) // same key in old and new
		nowIn := f.Rule.Matches(event.New)
		switch {
		case wasIn && nowIn:
			return domain.Delta{Modified: []domain.Row{event.New}}, true
		case wasIn && !nowIn:
			return domain.Delta{Removed: []domain.Row{old}}, true
		case !wasIn && nowIn:
			return domain.Delta{Added: []domain.Row{event.New}}, true
		}
	}

	// Handled, but nothing to send (row didn't match, or wasn't ours).
	return domain.Delta{}, true
}

// find looks up the saved row whose key matches the event row's key.
func (f Filter) find(current domain.ResultSet, row domain.Row) (domain.Row, bool) {
	k := row[f.Key]
	for _, r := range current {
		if r[f.Key] == k {
			return r, true
		}
	}
	return nil, false
}

// A Rule is a compiled WHERE condition we can run against a single row, in Go,
// without asking the database. It's either a Cmp (one comparison) or an And/Or
// of other Rules. The filter operator uses it to decide whether a changed row
// belongs in the result.
type Rule interface {
	Matches(row domain.Row) bool
}

// Cmp is one comparison — the three pieces lifted from a parsed A_Expr: a column,
// an operator, and a constant value (e.g. status = 'pending').
type Cmp struct {
	Column string
	Op     string
	Value  any
}

func (c Cmp) Matches(row domain.Row) bool {
	return compare(row[c.Column], c.Op, c.Value)
}

// And passes only if every sub-rule passes.
type And struct{ Rules []Rule }

func (a And) Matches(row domain.Row) bool {
	for _, r := range a.Rules {
		if !r.Matches(row) {
			return false
		}
	}
	return true
}

// True matches every row — the rule for a query with no WHERE at all.
type True struct{}

func (True) Matches(domain.Row) bool { return true }

// Or passes if any sub-rule passes.
type Or struct{ Rules []Rule }

func (o Or) Matches(row domain.Row) bool {
	for _, r := range o.Rules {
		if r.Matches(row) {
			return true
		}
	}
	return false
}

// compare runs one operator over a row value and a rule constant. Numbers are
// compared numerically (row values arrive as int64/float64, constants as int);
// otherwise we fall back to string comparison. We only support operators we can
// evaluate exactly like Postgres for these simple types — anything the planner
// can't compile never reaches here, it becomes a re-eval instead.
func compare(rowVal any, op string, ruleVal any) bool {
	if a, aok := toFloat(rowVal); aok {
		if b, bok := toFloat(ruleVal); bok {
			switch op {
			case "=":
				return a == b
			case "<>", "!=":
				return a != b
			case ">":
				return a > b
			case "<":
				return a < b
			case ">=":
				return a >= b
			case "<=":
				return a <= b
			}
		}
	}

	as, bs := fmt.Sprint(rowVal), fmt.Sprint(ruleVal)
	switch op {
	case "=":
		return as == bs
	case "<>", "!=":
		return as != bs
	case ">":
		return as > bs
	case "<":
		return as < bs
	case ">=":
		return as >= bs
	case "<=":
		return as <= bs
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}
