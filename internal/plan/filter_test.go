package plan_test

import (
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/plan"
)

// A single comparison: does the row's column satisfy the rule?
func TestCmp_Equals(t *testing.T) {
	r := plan.Cmp{Column: "status", Op: "=", Value: "pending"}

	if !r.Matches(domain.Row{"status": "pending"}) {
		t.Fatalf("expected a pending row to match status = 'pending'")
	}
	if r.Matches(domain.Row{"status": "succeeded"}) {
		t.Fatalf("expected a succeeded row NOT to match status = 'pending'")
	}
}

// Numbers compare numerically, even across int/float (rows come back as int64,
// the rule constant is an int) — 150 > 100 regardless of the Go types.
func TestCmp_GreaterThan(t *testing.T) {
	r := plan.Cmp{Column: "amount", Op: ">", Value: 100}

	if !r.Matches(domain.Row{"amount": int64(150)}) {
		t.Fatalf("expected amount 150 to match amount > 100")
	}
	if r.Matches(domain.Row{"amount": int64(50)}) {
		t.Fatalf("expected amount 50 NOT to match amount > 100")
	}
}

// AND: every sub-rule must pass.
func TestAnd(t *testing.T) {
	r := plan.And{Rules: []plan.Rule{
		plan.Cmp{Column: "amount", Op: ">", Value: 100},
		plan.Cmp{Column: "status", Op: "=", Value: "pending"},
	}}

	if !r.Matches(domain.Row{"amount": int64(150), "status": "pending"}) {
		t.Fatalf("expected both conditions satisfied to match")
	}
	if r.Matches(domain.Row{"amount": int64(150), "status": "succeeded"}) {
		t.Fatalf("expected a failed sub-rule to fail the AND")
	}
}

// OR: any sub-rule passing is enough.
func TestOr(t *testing.T) {
	r := plan.Or{Rules: []plan.Rule{
		plan.Cmp{Column: "status", Op: "=", Value: "pending"},
		plan.Cmp{Column: "status", Op: "=", Value: "failed"},
	}}

	if !r.Matches(domain.Row{"status": "failed"}) {
		t.Fatalf("expected 'failed' to match one side of the OR")
	}
	if r.Matches(domain.Row{"status": "succeeded"}) {
		t.Fatalf("expected 'succeeded' to match neither side of the OR")
	}
}
