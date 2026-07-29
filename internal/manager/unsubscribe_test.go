package manager_test

import (
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/manager"
)

func TestUnsubscribe_Existing_RemovesOnlyThatSubscription(t *testing.T) {
	m, client := setup()
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "orders", SQL: "SELECT 1"})
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "cart", SQL: "SELECT 2"})

	res := m.Unsubscribe(manager.UnsubscribeInput{ClientID: "c1", ID: "orders"})

	if !res.IsOk() {
		t.Fatalf("expected ok, got error: %v", res.Err)
	}
	if _, ok := client.Subs["orders"]; ok {
		t.Fatalf("expected 'orders' to be removed")
	}
	if _, ok := client.Subs["cart"]; !ok {
		t.Fatalf("expected 'cart' to remain untouched")
	}
	if len(client.Subs) != 1 {
		t.Fatalf("expected 1 subscription left, got %d", len(client.Subs))
	}
}

func TestUnsubscribe_EmptySubID_ReturnsErrorAndRemovesNothing(t *testing.T) {
	m, client := setup()
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "orders", SQL: "SELECT 1"})

	res := m.Unsubscribe(manager.UnsubscribeInput{ClientID: "c1", ID: ""})

	if res.Err != manager.ErrEmptySubID {
		t.Fatalf("expected ErrEmptySubID, got %v", res.Err)
	}
	if len(client.Subs) != 1 {
		t.Fatalf("expected subscription untouched, got %d", len(client.Subs))
	}
}

func TestUnsubscribe_UnknownClient_ReturnsError(t *testing.T) {
	m := manager.New() // nobody registered

	res := m.Unsubscribe(manager.UnsubscribeInput{ClientID: "ghost", ID: "orders"})

	if res.Err != manager.ErrClientNotFound {
		t.Fatalf("expected ErrClientNotFound, got %v", res.Err)
	}
}

func TestUnsubscribe_UnknownSub_ReturnsErrorAndRemovesNothing(t *testing.T) {
	m, client := setup()
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "orders", SQL: "SELECT 1"})

	res := m.Unsubscribe(manager.UnsubscribeInput{ClientID: "c1", ID: "does-not-exist"})

	if res.Err != manager.ErrSubNotFound {
		t.Fatalf("expected ErrSubNotFound, got %v", res.Err)
	}
	if len(client.Subs) != 1 {
		t.Fatalf("expected subscription untouched, got %d", len(client.Subs))
	}
}
