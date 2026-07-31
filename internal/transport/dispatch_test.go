package transport_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
	"github.com/Emmanuel-MacAnThony/current/internal/manager"
	"github.com/Emmanuel-MacAnThony/current/internal/transport"
)

// captureConn records every frame the dispatch sends.
type captureConn struct{ sent [][]byte }

func (c *captureConn) Send(msg []byte) error { c.sent = append(c.sent, msg); return nil }
func (c *captureConn) Close() error          { return nil }

// fakeRunner stands in for the real query runner: canned rows or an error, and it
// records whether it ran — so we can prove empty inputs never hit the DB.
type fakeRunner struct {
	rows   domain.ResultSet
	err    error
	called bool
	gotSQL string
}

func (f *fakeRunner) Run(sql string) (domain.ResultSet, error) {
	f.called = true
	f.gotSQL = sql
	return f.rows, f.err
}

// frame is the test's view of an outbound message.
type frame struct {
	Type    string           `json:"type"`
	ID      string           `json:"id"`
	Message string           `json:"message"`
	Rows    []map[string]any `json:"rows"`
}

func decode(t *testing.T, b []byte) frame {
	t.Helper()
	var f frame
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("sent frame is not valid json: %v", err)
	}
	return f
}

func setup() (*transport.Server, *domain.Client, *captureConn, *fakeRunner) {
	m := manager.New()
	conn := &captureConn{}
	client, _ := m.Register("c1", conn)
	runner := &fakeRunner{}
	return transport.NewServer(m, runner), client, conn, runner
}

func TestDispatch_ValidSubscribe_RunsQueryStoresResultSendsData(t *testing.T) {
	s, client, conn, runner := setup()
	runner.rows = domain.ResultSet{{"id": 1, "amount": 1500}}

	s.HandleMessage("c1", conn, []byte(`{"type":"subscribe","id":"orders","sql":"SELECT id FROM orders"}`))

	if !runner.called || runner.gotSQL != "SELECT id FROM orders" {
		t.Fatalf("expected the query to run with the sql; called=%v sql=%q", runner.called, runner.gotSQL)
	}
	sub, ok := client.Subs["orders"]
	if !ok || len(sub.Result) != 1 || sub.Key != "id" {
		t.Fatalf("expected sub registered with the initial result and default key 'id', got %+v", sub)
	}
	if len(conn.sent) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(conn.sent))
	}
	f := decode(t, conn.sent[0])
	if f.Type != "data" || f.ID != "orders" || len(f.Rows) != 1 {
		t.Fatalf("expected a data frame with 1 row, got %+v", f)
	}
}

func TestDispatch_Subscribe_KeyOverride(t *testing.T) {
	s, client, conn, runner := setup()
	runner.rows = domain.ResultSet{{"pk": 7}}

	s.HandleMessage("c1", conn, []byte(`{"type":"subscribe","id":"orders","sql":"SELECT pk FROM t","key":"pk"}`))

	sub, ok := client.Subs["orders"]
	if !ok || sub.Key != "pk" {
		t.Fatalf("expected the client-declared key 'pk', got %+v", sub)
	}
}

func TestDispatch_SubscribeEmptySQL_ErrorsWithoutRunningQuery(t *testing.T) {
	s, client, conn, runner := setup()

	s.HandleMessage("c1", conn, []byte(`{"type":"subscribe","id":"orders","sql":""}`))

	if runner.called {
		t.Fatalf("should not run a query for empty sql")
	}
	if len(client.Subs) != 0 {
		t.Fatalf("expected nothing registered")
	}
	if f := decode(t, conn.sent[0]); f.Type != "error" || f.Message != manager.ErrEmptySQL.Error() {
		t.Fatalf("expected empty-sql error, got %+v", f)
	}
}

func TestDispatch_SubscribeQueryFails_ErrorsAndRegistersNothing(t *testing.T) {
	s, client, conn, runner := setup()
	runner.err = errors.New(`relation "orders" does not exist`)

	s.HandleMessage("c1", conn, []byte(`{"type":"subscribe","id":"orders","sql":"SELECT * FROM orders"}`))

	if len(client.Subs) != 0 {
		t.Fatalf("expected nothing registered on query failure")
	}
	if f := decode(t, conn.sent[0]); f.Type != "error" || f.ID != "orders" {
		t.Fatalf("expected error frame for the failed query, got %+v", f)
	}
}

func TestDispatch_MalformedJSON_SendsErrorFrame(t *testing.T) {
	s, client, conn, _ := setup()

	s.HandleMessage("c1", conn, []byte(`this is not json`))

	if len(client.Subs) != 0 {
		t.Fatalf("expected nothing registered")
	}
	if f := decode(t, conn.sent[0]); f.Type != "error" {
		t.Fatalf("expected error frame, got %+v", f)
	}
}

func TestDispatch_UnknownType_IsIgnored(t *testing.T) {
	s, client, conn, _ := setup()

	s.HandleMessage("c1", conn, []byte(`{"type":"whoami","id":"x"}`))

	if len(client.Subs) != 0 || len(conn.sent) != 0 {
		t.Fatalf("expected unknown type ignored: subs=%d sent=%d", len(client.Subs), len(conn.sent))
	}
}

func TestDispatch_ValidUnsubscribe_RemovesAndAcks(t *testing.T) {
	s, client, conn, _ := setup()
	s.HandleMessage("c1", conn, []byte(`{"type":"subscribe","id":"orders","sql":"SELECT 1"}`))
	conn.sent = nil // drop the subscribe data frame; we only care about the unsubscribe

	s.HandleMessage("c1", conn, []byte(`{"type":"unsubscribe","id":"orders"}`))

	if _, ok := client.Subs["orders"]; ok {
		t.Fatalf("expected subscription removed")
	}
	if f := decode(t, conn.sent[0]); f.Type != "ack" || f.ID != "orders" {
		t.Fatalf("expected ack for 'orders', got %+v", f)
	}
}
