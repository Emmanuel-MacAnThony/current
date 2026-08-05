package manager_test

import (
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/manager"
)

// hasSub reports whether the routed set contains a subscription with this subID.
func hasSub(refs []manager.SubRef, subID string) bool {
	for _, r := range refs {
		if r.SubID == subID {
			return true
		}
	}
	return false
}

// A subscription is indexed under every table its query reads, so a change to
// any one of them routes to it. The join case: reads two tables, findable by both.
func TestSubsForTable_IndexesSubUnderEveryTableItReads(t *testing.T) {
	m, _ := setup()
	m.Subscribe(manager.SubscribeInput{
		ClientID: "c1", ID: "join", SQL: "SELECT o.*, p.amount FROM orders o JOIN payments p ON ...",
		Key: "id", Tables: []string{"orders", "payments"},
	})

	if !hasSub(m.SubsForTable("orders"), "join") {
		t.Fatalf("expected 'join' routed under orders, got %+v", m.SubsForTable("orders"))
	}
	if !hasSub(m.SubsForTable("payments"), "join") {
		t.Fatalf("expected 'join' routed under payments, got %+v", m.SubsForTable("payments"))
	}
}

// A change on a table nobody reads routes to nothing — the whole point of the
// index (no scan, and an empty result the engine simply skips).
func TestSubsForTable_UnreadTable_ReturnsNothing(t *testing.T) {
	m, _ := setup()
	m.Subscribe(manager.SubscribeInput{
		ClientID: "c1", ID: "o", SQL: "SELECT id FROM orders", Key: "id", Tables: []string{"orders"},
	})

	if got := m.SubsForTable("payments"); len(got) != 0 {
		t.Fatalf("expected no subs for an unread table, got %+v", got)
	}
}

// Unsubscribe must pull the sub out of every bucket it was in, or the index
// leaks references to a sub that no longer exists.
func TestUnsubscribe_RemovesSubFromIndex(t *testing.T) {
	m, _ := setup()
	m.Subscribe(manager.SubscribeInput{
		ClientID: "c1", ID: "join", SQL: "SELECT ... FROM orders JOIN payments ...",
		Key: "id", Tables: []string{"orders", "payments"},
	})

	m.Unsubscribe(manager.UnsubscribeInput{ClientID: "c1", ID: "join"})

	if len(m.SubsForTable("orders")) != 0 || len(m.SubsForTable("payments")) != 0 {
		t.Fatalf("expected index cleared after unsubscribe, got orders=%+v payments=%+v",
			m.SubsForTable("orders"), m.SubsForTable("payments"))
	}
}

// A disconnecting client takes all its subs down with it — including out of the
// index. Otherwise SubsForTable hands the engine dead references.
func TestUnregister_RemovesAllClientSubsFromIndex(t *testing.T) {
	m, _ := setup()
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "a", SQL: "SELECT id FROM orders", Key: "id", Tables: []string{"orders"}})
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "b", SQL: "SELECT id FROM payments", Key: "id", Tables: []string{"payments"}})

	m.Unregister("c1")

	if len(m.SubsForTable("orders")) != 0 || len(m.SubsForTable("payments")) != 0 {
		t.Fatalf("expected index cleared after unregister, got orders=%+v payments=%+v",
			m.SubsForTable("orders"), m.SubsForTable("payments"))
	}
}

// Two subs reading the same table both route; removing one leaves the other —
// the bucket is a set of subs, not a single slot.
func TestSubsForTable_MultipleSubsSameTable(t *testing.T) {
	m, _ := setup()
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "a", SQL: "SELECT id FROM orders", Key: "id", Tables: []string{"orders"}})
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "b", SQL: "SELECT count(*) FROM orders", Key: "id", Tables: []string{"orders"}})

	if got := m.SubsForTable("orders"); len(got) != 2 {
		t.Fatalf("expected 2 subs routed for orders, got %+v", got)
	}

	m.Unsubscribe(manager.UnsubscribeInput{ClientID: "c1", ID: "a"})

	got := m.SubsForTable("orders")
	if len(got) != 1 || !hasSub(got, "b") {
		t.Fatalf("expected only 'b' left for orders, got %+v", got)
	}
}
