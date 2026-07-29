package manager_test

import (
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/manager"
)

func TestUnregister_RemovesClientAndItsSubscriptions(t *testing.T) {
	m, _ := setup()
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "orders", SQL: "SELECT 1"})

	m.Unregister("c1")

	// The client (and its subs, which lived under it) is gone: subscribing to it
	// now fails as if it never connected.
	res := m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "x", SQL: "SELECT 1"})
	if res.Err != manager.ErrClientNotFound {
		t.Fatalf("expected client gone (ErrClientNotFound), got %v", res.Err)
	}
}

func TestUnregister_UnknownClient_IsNoOpAndLeavesOthers(t *testing.T) {
	m, client := setup()
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "orders", SQL: "SELECT 1"})

	m.Unregister("ghost") // unknown — must be a harmless no-op

	// c1 untouched: still registered, still holding its sub.
	res := m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "cart", SQL: "SELECT 2"})
	if !res.IsOk() {
		t.Fatalf("unregistering an unknown client must not affect c1, got %v", res.Err)
	}
	if len(client.Subs) != 2 {
		t.Fatalf("c1 should still have its subs, got %d", len(client.Subs))
	}
}

func TestUnregister_Twice_IsIdempotent(t *testing.T) {
	m, _ := setup()

	m.Unregister("c1")
	m.Unregister("c1") // again — must not panic

	res := m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "x", SQL: "SELECT 1"})
	if res.Err != manager.ErrClientNotFound {
		t.Fatalf("expected client still gone, got %v", res.Err)
	}
}
