package plan_test

import (
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/plan"
)

// The filter for `WHERE status = 'pending'`, and a saved list of two pending rows.
func pendingFilter() (plan.Filter, domain.ResultSet) {
	f := plan.Filter{Rule: plan.Cmp{Column: "status", Op: "=", Value: "pending"}, Key: "id"}
	current := domain.ResultSet{
		{"id": int64(1), "status": "pending"},
		{"id": int64(2), "status": "pending"},
	}
	return f, current
}

func onlyKey(rows []domain.Row, key string, want any) bool {
	return len(rows) == 1 && rows[0][key] == want
}

func TestFilter_InsertMatching_Adds(t *testing.T) {
	f, current := pendingFilter()
	d, ok := f.Apply(domain.ChangeEvent{Op: domain.OpInsert, New: domain.Row{"id": int64(3), "status": "pending"}}, current)

	if !ok {
		t.Fatal("filter always handles the event")
	}
	if !onlyKey(d.Added, "id", int64(3)) || len(d.Removed) != 0 || len(d.Modified) != 0 {
		t.Fatalf("expected only id 3 added, got %+v", d)
	}
}

func TestFilter_InsertNonMatching_NoChange(t *testing.T) {
	f, current := pendingFilter()
	d, _ := f.Apply(domain.ChangeEvent{Op: domain.OpInsert, New: domain.Row{"id": int64(4), "status": "succeeded"}}, current)

	if !d.IsEmpty() {
		t.Fatalf("a non-matching insert changes nothing, got %+v", d)
	}
}

func TestFilter_DeleteInList_Removes(t *testing.T) {
	f, current := pendingFilter()
	// A delete event carries only the key of the gone row.
	d, _ := f.Apply(domain.ChangeEvent{Op: domain.OpDelete, Old: domain.Row{"id": int64(2)}}, current)

	if !onlyKey(d.Removed, "id", int64(2)) || len(d.Added) != 0 {
		t.Fatalf("expected id 2 removed, got %+v", d)
	}
	// The removed row is the one from the saved list (full row), not the sparse event.
	if d.Removed[0]["status"] != "pending" {
		t.Fatalf("expected the removed row to be the stored row, got %+v", d.Removed[0])
	}
}

func TestFilter_DeleteNotInList_NoChange(t *testing.T) {
	f, current := pendingFilter()
	d, _ := f.Apply(domain.ChangeEvent{Op: domain.OpDelete, Old: domain.Row{"id": int64(99)}}, current)

	if !d.IsEmpty() {
		t.Fatalf("deleting a row we never showed changes nothing, got %+v", d)
	}
}

func TestFilter_UpdateStaysIn_Modifies(t *testing.T) {
	f, current := pendingFilter()
	d, _ := f.Apply(domain.ChangeEvent{Op: domain.OpUpdate, New: domain.Row{"id": int64(1), "status": "pending"}}, current)

	if !onlyKey(d.Modified, "id", int64(1)) || len(d.Added) != 0 || len(d.Removed) != 0 {
		t.Fatalf("expected id 1 modified, got %+v", d)
	}
}

func TestFilter_UpdateLeaves_Removes(t *testing.T) {
	f, current := pendingFilter()
	// id 1 was pending (in the list), now succeeded → it leaves the result.
	d, _ := f.Apply(domain.ChangeEvent{Op: domain.OpUpdate, New: domain.Row{"id": int64(1), "status": "succeeded"}}, current)

	if !onlyKey(d.Removed, "id", int64(1)) || len(d.Added) != 0 || len(d.Modified) != 0 {
		t.Fatalf("expected id 1 removed, got %+v", d)
	}
}

func TestFilter_UpdateEnters_Adds(t *testing.T) {
	f, current := pendingFilter()
	// id 5 wasn't in the list, updated to pending → it enters the result.
	d, _ := f.Apply(domain.ChangeEvent{Op: domain.OpUpdate, New: domain.Row{"id": int64(5), "status": "pending"}}, current)

	if !onlyKey(d.Added, "id", int64(5)) || len(d.Removed) != 0 || len(d.Modified) != 0 {
		t.Fatalf("expected id 5 added, got %+v", d)
	}
}

func TestFilter_UpdateStaysOut_NoChange(t *testing.T) {
	f, current := pendingFilter()
	d, _ := f.Apply(domain.ChangeEvent{Op: domain.OpUpdate, New: domain.Row{"id": int64(5), "status": "succeeded"}}, current)

	if !d.IsEmpty() {
		t.Fatalf("a row that was out and stays out changes nothing, got %+v", d)
	}
}
