package manager_test

import (
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/manager"
)

// fakeConn is a stand-in for a real websocket connection. Subscribe never sends
// anything (errors go back via the transport, not the manager), so it does
// nothing — it only exists so a Client can hold a Connection.
type fakeConn struct{}

func (fakeConn) Send([]byte) error { return nil }
func (fakeConn) Close() error      { return nil }

// setup returns a manager plus its one already-registered client "c1" — the
// state the subscribe contract assumes (registration happens on handshake
// first). Register hands back the client so tests can inspect its Subs; it's the
// same object the flows mutate in the map.
func setup() (*manager.Manager, *domain.Client) {
	m := manager.New()
	client, _ := m.Register("c1", fakeConn{})
	return m, client
}

func TestSubscribe_ValidInput_RegistersSubscriptionWithInitialResult(t *testing.T) {
	m, client := setup()
	rows := domain.ResultSet{{"id": 1, "amount": 1500}}

	res := m.Subscribe(manager.SubscribeInput{
		ClientID: "c1", ID: "orders", SQL: "SELECT id FROM orders", Key: "id", Result: rows,
	})

	if !res.IsOk() {
		t.Fatalf("expected ok, got error: %v", res.Err)
	}
	sub, ok := client.Subs["orders"]
	if !ok {
		t.Fatalf("expected subscription 'orders' to be registered")
	}
	if sub.ID != "orders" || sub.ClientID != "c1" || sub.SQL != "SELECT id FROM orders" || sub.Key != "id" {
		t.Fatalf("stored subscription is wrong: %+v", sub)
	}
	// The initial result is stored as the subscription's Memory (the "before" for
	// future diffs).
	if len(sub.Result) != 1 || sub.Result[0]["id"] != 1 {
		t.Fatalf("expected initial result stored, got %+v", sub.Result)
	}
	if len(client.Subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(client.Subs))
	}
}

func TestSubscribe_EmptySubID_ReturnsErrorAndRegistersNothing(t *testing.T) {
	m, client := setup()

	res := m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "", SQL: "SELECT 1"})

	if res.Err != manager.ErrEmptySubID {
		t.Fatalf("expected ErrEmptySubID, got %v", res.Err)
	}
	if len(client.Subs) != 0 {
		t.Fatalf("expected no subscriptions registered, got %d", len(client.Subs))
	}
}

func TestSubscribe_EmptySQL_ReturnsErrorAndRegistersNothing(t *testing.T) {
	m, client := setup()

	res := m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "orders", SQL: ""})

	if res.Err != manager.ErrEmptySQL {
		t.Fatalf("expected ErrEmptySQL, got %v", res.Err)
	}
	if len(client.Subs) != 0 {
		t.Fatalf("expected no subscriptions registered, got %d", len(client.Subs))
	}
}

func TestSubscribe_UnknownClient_ReturnsError(t *testing.T) {
	m := manager.New() // nobody registered

	res := m.Subscribe(manager.SubscribeInput{ClientID: "ghost", ID: "orders", SQL: "SELECT 1"})

	if res.Err != manager.ErrClientNotFound {
		t.Fatalf("expected ErrClientNotFound, got %v", res.Err)
	}
}

func TestSubscribe_DuplicateID_ReturnsErrorAndKeepsOriginal(t *testing.T) {
	m, client := setup()

	first := m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "orders", SQL: "SELECT 1"})
	if !first.IsOk() {
		t.Fatalf("setup: first subscribe should succeed, got %v", first.Err)
	}

	second := m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "orders", SQL: "SELECT 2"})

	if second.Err != manager.ErrDuplicateSub {
		t.Fatalf("expected ErrDuplicateSub, got %v", second.Err)
	}
	if got := client.Subs["orders"].SQL; got != "SELECT 1" {
		t.Fatalf("original subscription should be untouched, sql = %q", got)
	}
	if len(client.Subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(client.Subs))
	}
}
