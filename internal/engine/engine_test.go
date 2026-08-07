package engine_test

import (
	"context"
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/engine"
	"github.com/Emmanuel-MacAnThony/current/internal/manager"
	"github.com/Emmanuel-MacAnThony/current/internal/plan"
)

type fakeConn struct{}

func (fakeConn) Send([]byte) error { return nil }
func (fakeConn) Close() error      { return nil }

// fakeRunner returns a canned result for every re-run (the query text is ignored),
// and records whether it was called — so the filter test can prove no DB hit.
type fakeRunner struct {
	rows   domain.ResultSet
	err    error
	called bool
}

func (f *fakeRunner) Run(sql string) (domain.ResultSet, error) { f.called = true; return f.rows, f.err }

// fakePusher records every diff pushed, so tests can assert what went out.
type push struct {
	subID string
	delta domain.Delta
}
type fakePusher struct{ pushes []push }

func (p *fakePusher) Push(conn domain.Connection, subID string, delta domain.Delta) {
	p.pushes = append(p.pushes, push{subID, delta})
}

// fakeSource feeds the engine canned change events, then returns (so Run ends).
type fakeSource struct{ events []domain.ChangeEvent }

func (f *fakeSource) Run(ctx context.Context, handle func(domain.ChangeEvent)) error {
	for _, e := range f.events {
		handle(e)
	}
	return nil
}

func TestEngine_Change_RerunsDiffsAndPushes(t *testing.T) {
	m := manager.New()
	client, _ := m.Register("c1", fakeConn{})
	old := domain.ResultSet{{"id": 1, "status": "pending"}, {"id": 2, "status": "pending"}}
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "orders", SQL: "SELECT ...", Key: "id", Result: old, Tables: []string{"orders"}, Operator: plan.ReEval{}})

	// After the change, the query now returns only id 1 (id 2 shipped out of the set).
	next := domain.ResultSet{{"id": 1, "status": "pending"}}
	runner := &fakeRunner{rows: next}
	pusher := &fakePusher{}
	source := &fakeSource{events: []domain.ChangeEvent{{Table: "orders", Op: domain.OpUpdate}}}

	eng := engine.New(source, m, runner, pusher)
	_ = eng.Run(context.Background())

	// Memory is overwritten with the fresh result.
	if got := client.Subs["orders"].Result; len(got) != 1 {
		t.Fatalf("expected Memory updated to 1 row, got %+v", got)
	}
	// A single diff was pushed: id 2 removed.
	if len(pusher.pushes) != 1 {
		t.Fatalf("expected 1 push, got %d", len(pusher.pushes))
	}
	p := pusher.pushes[0]
	if p.subID != "orders" || len(p.delta.Removed) != 1 || p.delta.Removed[0]["id"] != 2 {
		t.Fatalf("expected id 2 removed pushed to 'orders', got %+v", p)
	}
}

func TestEngine_NoChange_DoesNotPush(t *testing.T) {
	m := manager.New()
	m.Register("c1", fakeConn{})
	same := domain.ResultSet{{"id": 1, "status": "pending"}}
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "orders", SQL: "SELECT ...", Key: "id", Result: same, Tables: []string{"orders"}, Operator: plan.ReEval{}})

	// The re-run returns exactly the current result → empty delta → nothing to push.
	runner := &fakeRunner{rows: domain.ResultSet{{"id": 1, "status": "pending"}}}
	pusher := &fakePusher{}
	source := &fakeSource{events: []domain.ChangeEvent{{Table: "orders"}}}

	eng := engine.New(source, m, runner, pusher)
	_ = eng.Run(context.Background())

	if len(pusher.pushes) != 0 {
		t.Fatalf("expected no push for an unchanged result, got %d", len(pusher.pushes))
	}
}

// A Filter sub handles an insert entirely in memory: the row is added to Memory,
// a diff is pushed, and the query runner is NEVER called (no DB round-trip).
func TestEngine_FilterInsert_UpdatesInMemoryNoQuery(t *testing.T) {
	m := manager.New()
	client, _ := m.Register("c1", fakeConn{})
	old := domain.ResultSet{{"id": int64(1), "status": "pending"}}
	filter := plan.Filter{Rule: plan.Cmp{Column: "status", Op: "=", Value: "pending"}, Key: "id"}
	m.Subscribe(manager.SubscribeInput{ClientID: "c1", ID: "pend", SQL: "SELECT ...", Key: "id", Result: old, Tables: []string{"payments"}, Operator: filter})

	runner := &fakeRunner{} // if it's ever called, the filter path is broken
	pusher := &fakePusher{}
	source := &fakeSource{events: []domain.ChangeEvent{
		{Table: "payments", Op: domain.OpInsert, New: domain.Row{"id": int64(2), "status": "pending"}},
	}}

	eng := engine.New(source, m, runner, pusher)
	_ = eng.Run(context.Background())

	if runner.called {
		t.Fatal("filter path must not hit the query runner")
	}
	// Memory grew to 2 rows, in memory.
	if got := client.Subs["pend"].Result; len(got) != 2 {
		t.Fatalf("expected Memory grown to 2 rows via the filter, got %+v", got)
	}
	// A diff was pushed: id 2 added.
	if len(pusher.pushes) != 1 {
		t.Fatalf("expected 1 push, got %d", len(pusher.pushes))
	}
	if d := pusher.pushes[0].delta; len(d.Added) != 1 || d.Added[0]["id"] != int64(2) {
		t.Fatalf("expected id 2 added pushed, got %+v", d)
	}
}
