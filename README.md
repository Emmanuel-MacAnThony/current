# Current — Live Query Engine

> Subscribe to a SQL query. Whenever the data changes, get pushed a **diff** of what changed in your result — not the whole result, not a manual event. The UI stays live without anyone wiring up notifications by hand.

---

## Demo

A page subscribes to `SELECT id, status, amount, currency FROM payments ORDER BY id` over one WebSocket. The engine runs it once and sends the initial rows; from then on it watches Postgres's change log and pushes only what changed.

<p align="center">
  <img src="web/screenshots/01-dashboard.png" alt="Current dashboard — the live result of a subscribed SQL query, connected over WebSocket" width="100%"/>
</p>

Then a row is changed **directly in the database** — `UPDATE payments SET status='failed' …`, no application code, no emitted event. The engine sees it on the WAL, re-evaluates the query, diffs it against what the client last saw, and pushes just that one modified row. The client patches its table and the row flashes; the push shows up in the log as `diff pushed — ~1`:

<p align="center">
  <img src="web/screenshots/02-live-diff.png" alt="A row changed in Postgres is pushed to the client as a diff and flashes in the table" width="100%"/>
</p>

Nothing polled, nothing hand-wired — the database was the source of truth for "did this change?", exactly as intended.

> The demo UI lives in [`web/`](web/) (Next.js). The full change-flow is built and verified against Postgres logical replication: the **matchmaker** routes each change only to the subscriptions that read the changed table, and a simple `SELECT *` filter is maintained **incrementally in memory** — no DB round-trip; everything else falls back to a correct **re-eval**. (Deferred: incremental aggregate/`max` operators, and projected-column filters — see Status below.)

**Run it yourself** — the whole demo (seeded Postgres + engine + UI) is one command:

```bash
docker compose up --build      # then open http://localhost:3002
```

Then change a row in the bundled database and watch it appear on the page:

```bash
docker compose exec db psql -U current -d current \
  -c "update payments set status='failed' where id='pay_001'"
```

<details>
<summary>Run the pieces directly (no Docker), against any <code>wal_level=logical</code> Postgres</summary>

```bash
# the engine — watches the DB, serves the WebSocket on :8080
DATABASE_URL="postgres://user:pass@localhost:5432/db" go run ./cmd/current

# the demo UI — http://localhost:3002
cd web && npm install && npm run dev
```
</details>

---

## The problem

Real-time UI today is hand-wired. Every place your code changes data, it also has to remember to announce it — `socket.emit('order-updated')` — so the screen updates. That couples two things that shouldn't be coupled:

- the **write side** has to know what the UI is showing (which events to fire), and
- the **UI** has to know which events map to its data.

Forget to emit on one code path — or run a script/job that writes directly — and the UI **silently goes stale**. No error. The screen is just wrong, and nobody knows.

## What we're building

**Decouple "what changed in the data" from "who needs to know."**

Instead of making every writer announce its changes, a client **subscribes to a query** — `SELECT * FROM orders WHERE status='pending'` — and the engine automatically pushes a diff (`{added, removed, modified}`) whenever anything changes that result set. The writers go back to just writing. The source of truth for "did this change?" becomes the **database itself**, observed directly.

Correctness that **can't be forgotten**: the screen is right no matter *how* the data changed.

*(This category is real — Materialize, ReadySet, Supabase Realtime, RethinkDB. Current is a miniature of that idea, built to understand it.)*

---

## What it is — a standalone, stateless service

Current is **internal infrastructure**, not a product and not code embedded in your backend. It's its own service that:

- **Sits next to one database** — configured at deploy time (`DATABASE_URL=… ./current`); one instance watches one DB. The browser never sees the database; that connection is a private, server-side detail.
- **The browser talks to it directly** over WebSocket — your app's backend isn't in the live-query path at all.
- **Replaces WebSocket spaghetti** — instead of every team hand-writing `emit` calls, a client subscribes to a SQL query and gets live diffs.

### The interaction flow

**Setup (once, by ops):** run the engine with the company's `DATABASE_URL` → it connects to their DB and attaches its replication slot.

**Runtime (per user, in the React app):**
1. React app opens a WebSocket to the engine (carrying an auth token).
2. A component mounts (e.g. the orders dashboard) → sends `subscribe("SELECT … WHERE status='pending'")`.
3. Engine runs it once → returns the **initial rows** → component renders them.
4. DB changes → engine pushes a **diff** → component patches its rows.
5. Component unmounts → sends `unsubscribe`.
6. Connection drops → React **auto-reconnects** → **re-subscribes** whatever components are still mounted.

### State & persistence

The service **persists nothing of its own.** It holds live, in-memory, connection-scoped state (subscriptions, plans, each one's current result) **only while connected**, and throws it all away when the connection drops. On restart it comes up empty and rebuilds as clients reconnect.

- **The React app is the memory** of what to watch — its mounted components *are* the subscription list, and it re-declares them on reconnect. So there's no client/subscription state to store.
- **The only durable state** is the company's **database** (their data, not ours) and the **Postgres replication slot** (the "how far have I read the log" cursor — Postgres keeps it, not us).

So the model flows *in* from the React app: it declares what to watch, we build the domain on demand (`Client → Subscription → Plan → Memory`), serve live diffs, and discard it on disconnect.

---

## Requirements — the database

Current is a live view over **one existing Postgres**, configured at startup via `DATABASE_URL` (never by clients). That database has one hard requirement:

> **`wal_level = logical`**

Current reads the database's change stream via **logical replication** (the same mechanism Debezium and Supabase Realtime use), and that stream is only *readable* when `wal_level = logical`. At the default `replica`, Postgres doesn't write the detail an outside reader needs — there's nothing to decode.

What this means for adopting it:
- **It's a config change + a one-time restart — not a data migration.** `wal_level` is server config; your data is untouched.
- **It's standard for this whole category.** Every CDC / live-query tool requires it, and many production databases already run it.

Current **fails fast**: at startup it runs `SHOW wal_level` and refuses to start (with a clear message) if it isn't `logical`. It can *validate* the setting but can't enable it for you.

**Enabling it:**
- **Docker Postgres:** add `command: ["postgres", "-c", "wal_level=logical"]` to the service and restart the container — a named volume keeps your data.
- **Managed (RDS / Cloud SQL):** set the logical-replication parameter and reboot once.

*(Full **old**-row values on updates/deletes additionally need `REPLICA IDENTITY FULL` on the watched tables — this matters for diffing, and is noted where it comes up.)*

---

## How it works — the mental model

### The components

| Role | One-line job |
|---|---|
| **Subscription Manager** | Takes a client's "watch this query," returns the starting result. |
| **Planner** | Compiles the query into a **Plan** (a chain of small operators in SQL's order) and notes which tables it reads. |
| **Watcher** | Reads the database's change stream and emits each change as an event. |
| **Matchmaker** | Routes each event to the subscriptions that read that table. |
| **Plan** | The chain of operators. A change flows **up** through it; each operator updates its piece or re-reads if stuck; the top emits the diff. |
| **Memory** | Holds each subscription's current result (the "before" a diff compares against). |
| **Messenger** | Pushes the diff to the client over its open connection. |

### Two phases

**Subscribe (once per query):**
```
UI → Subscription Manager → Planner (builds the Plan + table list)
                          → run query once → Memory → send initial result → UI
```

**React to a change (forever):**
```
Postgres → Watcher → Matchmaker → Plan (change flows up the operators)
                                → Memory (update) → Messenger → UI
```

### The whole system

```
        ┌─────────────────────────── UI (browser) ───────────────────────────┐
        │   subscribe(sql) ▲                              ▲ patch rows        │
        └──────────┬───────┴──────────────────────────────┴──────────────────┘
                   │ WebSocket                              ▲ WebSocket
                   ▼                                        │
        ┌───────────────────┐                      ┌────────────────┐
        │  Subscription Mgr │                      │   Messenger    │
        └─────────┬─────────┘                      └───────▲────────┘
                  ▼                                         │
             ┌─────────┐   initial run    ┌──────────┐     │
             │ Planner │ ───────────────► │  Memory  │ ────┘
             └────┬────┘                  └────▲─────┘
                  │ Plan+tables               │ update
                  ▼                           │
        ══════ per-subscription ══════   ┌────┴────┐   route   ┌────────────┐
                                         │  Plan   │ ◄──────── │ Matchmaker │
                                         └────▲────┘           └─────▲──────┘
                                              │ re-read              │ event
                                         ┌────┴─────┐          ┌─────┴────┐
                                         │ Postgres │ ───────► │ Watcher  │
                                         └──────────┘  logical └──────────┘
                                                       stream
```

**Straight-line summary**
- **Subscribe (down):** `UI → Subscription Manager → Planner → Postgres → Memory → UI`
- **Change (up):** `Postgres → Watcher → Matchmaker → Plan → Memory → Messenger → UI`

---

## Key design decisions (the *why*)

**1. Hook the change-log, don't poll.**
Polling (re-run queries on a timer) has designed-in lag, wastes work, and *misses history* (a row that changes twice between polls looks like it changed once). Instead we tap the database's own **write-ahead log** via **logical replication** — it already records every change, in order, durably, for its own crash-recovery and replicas. We consume the *logical* (decoded) stream: `{table, op, old row, new row}`. Zero extra cost to writes, never misses a change.

**2. A change either updates the result in place, or forces a re-read.**
The rule: a query can be maintained incrementally as long as **`new_result = f(stored_result, event)`** — the change plus what we already hold is enough to know the new answer. If it needs information in neither, we go back to the database. It's not about the query's *shape* — it's about *sufficiency of information*.

**3. Compile each query into a Plan of operators — don't judge the whole query by its worst part.**
A query is a chain of steps (`Filter → Count`, `Join → Filter → Max`, in SQL's fixed order). Each operator does its own cheap update. "Count of pending orders" stays `+1 / −1` in memory instead of re-running the DB just because it contains an aggregate. Operators are reusable pieces you snap together — so you don't need a bespoke handler for every query combination.

**4. Re-eval is the catch-all fallback, and it's just another operator.**
Anything the Planner can't decompose collapses into a single **ReEval** operator that re-runs the query and diffs against Memory (via a generic `compareByKey`). Always correct, for any query. So **correctness never depends on covering all of SQL** — unrecognized clauses safely fall through to re-eval. Fast paths are an opt-in allow-list on top.

**5. Additions are cheap; a removal that leaves a gap is what sends you to the DB.**
`count`/`sum` never re-read (they don't care *which* row left). `max`/`top-N` re-read only when the row with a *specific role* leaves and its successor was discarded — "what's behind the thing that just left?" is a question only the source can answer. That's the exact line where in-memory maintenance ends and the database begins.

---

## Status — what's built

| Piece | State |
|---|---|
| Watcher (logical replication → change events) | ✅ built |
| Subscription manager (register / subscribe / unsubscribe, connection lifecycle) | ✅ built |
| Query-run on subscribe + initial rows | ✅ built |
| Diff by key + push over WebSocket | ✅ built |
| Engine change-flow (re-evaluate → diff → push) | ✅ built |
| **Matchmaker** (route a change only to subs that read the table) | ✅ built |
| **Filter operator** (maintain a `SELECT *` filter in memory, no DB) | ✅ built |
| **ReEval operator** (correct fallback for everything else) | ✅ built |
| Demo UI (Next.js) + self-contained `docker compose` | ✅ built |
| `count` / `sum` / `max` / `top-N` operators | ⬜ deferred |
| Projected-column filters (`SELECT a, b …`) | ⬜ deferred (fall back to re-eval today) |
| Snapshot↔stream boundary, reconnect/resume, auth | ⬜ deferred |

The **operator interface** is the extension point: each new operator (count, max, …) is a new `Apply` implementation the planner can select; anything unhandled stays correct via ReEval.

---

## Tech stack

| Component | Tool |
|---|---|
| Database | PostgreSQL, `wal_level=logical` + a publication + replication slot |
| Watcher | Go + `pglogrepl` (via `pgx`) on the logical replication slot |
| Planner | Go + `pg_query_go` (Postgres's own parser, via cgo — linked static against musl) |
| Matchmaker | In-memory index — Go map `table → []subscription` |
| Plan / operators | Go (custom operators sharing one `Apply` interface) |
| Memory | In-process Go maps (→ Redis if scaled to many nodes) |
| Subscription Manager / Messenger | Go + WebSocket (`coder/websocket`) |
| UI | Browser — React/JS holding a WebSocket, patches rows on each diff |

**Language:** Go throughout, Postgres as the source of truth.
