package diff_test

import (
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/diff"
	"github.com/Emmanuel-MacAnThony/current/internal/domain"
)

func TestByKey_Added(t *testing.T) {
	old := domain.ResultSet{{"id": 1, "status": "a"}}
	next := domain.ResultSet{{"id": 1, "status": "a"}, {"id": 2, "status": "b"}}

	d := diff.ByKey(old, next, "id")

	if len(d.Added) != 1 || d.Added[0]["id"] != 2 {
		t.Fatalf("expected id 2 added, got %+v", d.Added)
	}
	if len(d.Removed) != 0 || len(d.Modified) != 0 {
		t.Fatalf("expected only an add, got %+v", d)
	}
}

func TestByKey_Removed(t *testing.T) {
	old := domain.ResultSet{{"id": 1}, {"id": 2}}
	next := domain.ResultSet{{"id": 1}}

	d := diff.ByKey(old, next, "id")

	if len(d.Removed) != 1 || d.Removed[0]["id"] != 2 {
		t.Fatalf("expected id 2 removed, got %+v", d.Removed)
	}
	if len(d.Added) != 0 || len(d.Modified) != 0 {
		t.Fatalf("expected only a remove, got %+v", d)
	}
}

func TestByKey_Modified(t *testing.T) {
	old := domain.ResultSet{{"id": 1, "status": "pending"}}
	next := domain.ResultSet{{"id": 1, "status": "shipped"}}

	d := diff.ByKey(old, next, "id")

	if len(d.Modified) != 1 || d.Modified[0]["status"] != "shipped" {
		t.Fatalf("expected id 1 modified to shipped, got %+v", d.Modified)
	}
	if len(d.Added) != 0 || len(d.Removed) != 0 {
		t.Fatalf("expected only a modify, got %+v", d)
	}
}

func TestByKey_NoChange_IsEmpty(t *testing.T) {
	old := domain.ResultSet{{"id": 1, "status": "pending"}}
	next := domain.ResultSet{{"id": 1, "status": "pending"}}

	if d := diff.ByKey(old, next, "id"); !d.IsEmpty() {
		t.Fatalf("expected empty delta, got %+v", d)
	}
}

func TestByKey_Mixed(t *testing.T) {
	// id1 unchanged, id2 modified, id3 removed, id4 added
	old := domain.ResultSet{{"id": 1, "s": "a"}, {"id": 2, "s": "b"}, {"id": 3, "s": "c"}}
	next := domain.ResultSet{{"id": 1, "s": "a"}, {"id": 2, "s": "B"}, {"id": 4, "s": "d"}}

	d := diff.ByKey(old, next, "id")

	if len(d.Added) != 1 || d.Added[0]["id"] != 4 {
		t.Fatalf("expected id 4 added, got %+v", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0]["id"] != 3 {
		t.Fatalf("expected id 3 removed, got %+v", d.Removed)
	}
	if len(d.Modified) != 1 || d.Modified[0]["id"] != 2 || d.Modified[0]["s"] != "B" {
		t.Fatalf("expected id 2 modified to B, got %+v", d.Modified)
	}
}
