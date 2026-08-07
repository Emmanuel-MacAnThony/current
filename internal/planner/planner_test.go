package planner_test

import (
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/planner"
)

func TestCompile_SimpleEquals(t *testing.T) {
	r, ok := planner.Compile("SELECT * FROM payments WHERE status = 'pending'")
	if !ok {
		t.Fatal("expected status = 'pending' to compile")
	}
	if !r.Matches(domain.Row{"status": "pending"}) || r.Matches(domain.Row{"status": "succeeded"}) {
		t.Fatalf("compiled rule doesn't behave like status = 'pending'")
	}
}

func TestCompile_And(t *testing.T) {
	r, ok := planner.Compile("SELECT * FROM payments WHERE amount > 100 AND status = 'pending'")
	if !ok {
		t.Fatal("expected the AND to compile")
	}
	if !r.Matches(domain.Row{"amount": int64(150), "status": "pending"}) {
		t.Fatal("both conditions satisfied should match")
	}
	if r.Matches(domain.Row{"amount": int64(150), "status": "succeeded"}) {
		t.Fatal("a failed status should fail the AND")
	}
	if r.Matches(domain.Row{"amount": int64(50), "status": "pending"}) {
		t.Fatal("a failed amount should fail the AND")
	}
}

func TestCompile_NoWhere_MatchesEverything(t *testing.T) {
	r, ok := planner.Compile("SELECT * FROM payments")
	if !ok {
		t.Fatal("a query with no WHERE should compile to an always-true rule")
	}
	if !r.Matches(domain.Row{"anything": 1}) {
		t.Fatal("no WHERE should match every row")
	}
}

// A function call in the WHERE is a shape we don't evaluate — give up (→ re-eval).
func TestCompile_FunctionCall_GivesUp(t *testing.T) {
	if _, ok := planner.Compile("SELECT * FROM payments WHERE lower(user_id) = 'x'"); ok {
		t.Fatal("expected give-up for a function call in the WHERE")
	}
}

// A subquery in the WHERE is also out of scope — give up.
func TestCompile_Subquery_GivesUp(t *testing.T) {
	if _, ok := planner.Compile("SELECT * FROM payments WHERE id IN (SELECT id FROM other)"); ok {
		t.Fatal("expected give-up for a subquery in the WHERE")
	}
}

// Shape gate: a filter can only maintain `SELECT *`. These reshape the result and
// must fall back to re-eval, no matter how simple their WHERE is.

func TestCompile_ProjectedColumns_GivesUp(t *testing.T) {
	if _, ok := planner.Compile("SELECT id, status FROM payments WHERE status = 'pending'"); ok {
		t.Fatal("a projected column list can't be filter-maintained — expected give-up")
	}
}

func TestCompile_Aggregate_GivesUp(t *testing.T) {
	if _, ok := planner.Compile("SELECT count(*) FROM payments WHERE status = 'pending'"); ok {
		t.Fatal("an aggregate isn't a row set — expected give-up")
	}
}

func TestCompile_Limit_GivesUp(t *testing.T) {
	if _, ok := planner.Compile("SELECT * FROM payments LIMIT 3"); ok {
		t.Fatal("a LIMIT is a top-N, not filter-maintainable — expected give-up")
	}
}

func TestCompile_Join_GivesUp(t *testing.T) {
	if _, ok := planner.Compile("SELECT * FROM orders o JOIN payments p ON o.pid = p.id"); ok {
		t.Fatal("a join reads two tables — expected give-up")
	}
}
