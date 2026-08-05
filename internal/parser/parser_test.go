package parser_test

import (
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/parser"
)

func has(tables []string, name string) bool {
	for _, t := range tables {
		if t == name {
			return true
		}
	}
	return false
}

func TestTables_SimpleSelect(t *testing.T) {
	got, err := parser.Parser{}.Tables("SELECT id, status FROM payments ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "payments" {
		t.Fatalf("expected [payments], got %v", got)
	}
}

// A join reads both tables, so both must come out — that's the case where a
// change on one table has to wake a query "about" the other.
func TestTables_Join_CollectsBoth(t *testing.T) {
	got, err := parser.Parser{}.Tables("SELECT o.id, p.amount FROM orders o JOIN payments p ON o.pid = p.id")
	if err != nil {
		t.Fatal(err)
	}
	if !has(got, "orders") || !has(got, "payments") {
		t.Fatalf("expected orders and payments, got %v", got)
	}
}

// A table read only inside a subquery still counts — top-level FROM isn't enough.
func TestTables_Subquery_CollectsNested(t *testing.T) {
	got, err := parser.Parser{}.Tables("SELECT * FROM orders WHERE pid IN (SELECT id FROM payments WHERE status = 'failed')")
	if err != nil {
		t.Fatal(err)
	}
	if !has(got, "orders") || !has(got, "payments") {
		t.Fatalf("expected orders and payments, got %v", got)
	}
}

// The same table read twice (self-join) is one index entry, not two.
func TestTables_Dedupes(t *testing.T) {
	got, err := parser.Parser{}.Tables("SELECT * FROM payments a JOIN payments b ON a.id = b.parent_id")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "payments" {
		t.Fatalf("expected [payments] once, got %v", got)
	}
}

// Unparseable SQL surfaces an error — the caller rejects the subscribe instead of
// silently indexing under nothing (which would make the query never update).
func TestTables_InvalidSQL_Errors(t *testing.T) {
	if _, err := (parser.Parser{}).Tables("SELECT * FRO payments WHERE"); err == nil {
		t.Fatalf("expected a parse error for invalid SQL")
	}
}
