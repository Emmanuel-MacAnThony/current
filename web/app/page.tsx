"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Radio, RadioTower, Database, Play, ArrowRight } from "lucide-react";
import { cn } from "@/lib/utils";

// The row identity column. Our engine diffs by this key; the client applies the
// pushed diff the same way — match rows by key, then add / remove / replace.
const KEY = "id";
const SUB_ID = "s1";
const DEFAULT_SQL = "SELECT id, status, amount, currency FROM payments ORDER BY id";
const WS_URL = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080/ws";

type Row = Record<string, unknown>;

type Frame =
  | { type: "data"; id: string; rows?: Row[] }
  | { type: "diff"; id: string; added?: Row[]; removed?: Row[]; modified?: Row[] }
  | { type: "ack"; id?: string }
  | { type: "error"; id?: string; message?: string };

type FlashKind = "add" | "mod";
type LogEntry = { seq: number; kind: "data" | "diff" | "error"; text: string; time: string };

function keyOf(r: Row) {
  return String(r[KEY]);
}

// Apply a pushed diff to the current rows — the mirror of what the engine does
// server-side. This is the whole client: it never re-fetches, it just folds diffs.
function applyDiff(prev: Row[], d: Extract<Frame, { type: "diff" }>): Row[] {
  const byKey = new Map(prev.map((r) => [keyOf(r), r]));
  for (const r of d.removed ?? []) byKey.delete(keyOf(r));
  for (const r of d.added ?? []) byKey.set(keyOf(r), r);
  for (const r of d.modified ?? []) byKey.set(keyOf(r), r);
  return [...byKey.values()].sort((a, b) => keyOf(a).localeCompare(keyOf(b)));
}

function now() {
  return new Date().toLocaleTimeString("en-US", { hour12: false });
}

export default function Home() {
  const [connected, setConnected] = useState(false);
  const [sql, setSql] = useState(DEFAULT_SQL);
  const [activeSql, setActiveSql] = useState(DEFAULT_SQL);
  const [rows, setRows] = useState<Row[]>([]);
  const [flash, setFlash] = useState<Record<string, FlashKind>>({});
  const [log, setLog] = useState<LogEntry[]>([]);
  const [error, setError] = useState<string | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const rowsRef = useRef<Row[]>([]);
  const seqRef = useRef(0);
  rowsRef.current = rows;

  const pushLog = useCallback((kind: LogEntry["kind"], text: string) => {
    setLog((prev) => [{ seq: ++seqRef.current, kind, text, time: now() }, ...prev].slice(0, 10));
  }, []);

  const flashRows = useCallback((keys: string[], kind: FlashKind) => {
    if (keys.length === 0) return;
    setFlash((prev) => {
      const next = { ...prev };
      for (const k of keys) next[k] = kind;
      return next;
    });
    setTimeout(() => {
      setFlash((prev) => {
        const next = { ...prev };
        for (const k of keys) delete next[k];
        return next;
      });
    }, 2200);
  }, []);

  const subscribe = useCallback((query: string) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    setError(null);
    setRows([]);
    setActiveSql(query);
    // Re-point the same subscription id: drop the old one, open the new query.
    ws.send(JSON.stringify({ type: "unsubscribe", id: SUB_ID }));
    ws.send(JSON.stringify({ type: "subscribe", id: SUB_ID, sql: query, key: KEY }));
  }, []);

  // One socket for the page's lifetime; reconnects if it drops.
  useEffect(() => {
    let closed = false;
    let retry: ReturnType<typeof setTimeout>;

    function connect() {
      const ws = new WebSocket(WS_URL);
      wsRef.current = ws;

      ws.onopen = () => {
        setConnected(true);
        ws.send(JSON.stringify({ type: "subscribe", id: SUB_ID, sql: DEFAULT_SQL, key: KEY }));
        setActiveSql(DEFAULT_SQL);
      };
      ws.onclose = () => {
        setConnected(false);
        if (!closed) retry = setTimeout(connect, 1500);
      };
      ws.onerror = () => ws.close();
      ws.onmessage = (evt) => {
        let msg: Frame;
        try {
          msg = JSON.parse(evt.data);
        } catch {
          return;
        }
        if (msg.type === "data") {
          const next = msg.rows ?? [];
          setRows(next);
          pushLog("data", `initial snapshot — ${next.length} row${next.length === 1 ? "" : "s"}`);
        } else if (msg.type === "diff") {
          const added = msg.added ?? [];
          const removed = msg.removed ?? [];
          const modified = msg.modified ?? [];
          setRows(applyDiff(rowsRef.current, msg));
          flashRows(added.map(keyOf), "add");
          flashRows(modified.map(keyOf), "mod");
          const parts = [
            added.length && `+${added.length}`,
            modified.length && `~${modified.length}`,
            removed.length && `-${removed.length}`,
          ].filter(Boolean);
          pushLog("diff", `diff pushed — ${parts.join("  ") || "no change"}`);
        } else if (msg.type === "error") {
          setError(msg.message ?? "unknown error");
          pushLog("error", msg.message ?? "unknown error");
        }
      };
    }

    connect();
    return () => {
      closed = true;
      clearTimeout(retry);
      wsRef.current?.close();
    };
  }, [pushLog, flashRows]);

  // Rows arrive as JSON objects whose key order is alphabetical (Go marshals maps
  // with sorted keys), so we impose a friendlier order: identity first, then the
  // rest. Unknown columns fall in alphabetically after the preferred ones.
  const columns = useMemo(() => {
    const preferred = ["id", "status", "amount", "currency"];
    const keys = rows[0] ? Object.keys(rows[0]) : preferred;
    const lead = preferred.filter((k) => keys.includes(k));
    const rest = keys.filter((k) => !preferred.includes(k)).sort();
    return [...lead, ...rest];
  }, [rows]);

  return (
    <main className="mx-auto max-w-5xl px-6 py-10">
      {/* Header */}
      <header className="mb-8 flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-xl border border-border bg-card">
            <RadioTower className="size-5 text-primary" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">Current</h1>
            <p className="text-sm text-muted-foreground">Live SQL over WebSocket — the server pushes diffs when the result changes.</p>
          </div>
        </div>
        <span
          className={cn(
            "inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset",
            connected
              ? "bg-primary/10 text-primary ring-primary/20"
              : "bg-muted text-muted-foreground ring-border",
          )}
        >
          <Radio className={cn("size-3", connected && "pulse-dot")} />
          {connected ? "Live" : "Connecting…"}
        </span>
      </header>

      {/* Query bar */}
      <div className="mb-5">
        <label className="mb-1.5 flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">
          <Database className="size-3.5" /> subscription query
        </label>
        <div className="flex gap-2">
          <input
            value={sql}
            onChange={(e) => setSql(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && subscribe(sql)}
            spellCheck={false}
            className="flex-1 rounded-lg border border-border bg-card px-3 py-2.5 font-mono text-sm text-foreground outline-none transition-colors focus:border-primary/60"
          />
          <button
            onClick={() => subscribe(sql)}
            disabled={!connected}
            className="inline-flex items-center gap-1.5 rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-40"
          >
            <Play className="size-3.5" /> Subscribe
          </button>
        </div>
        {error && <p className="mt-2 font-mono text-xs text-destructive">{error}</p>}
      </div>

      {/* Live table */}
      <div className="overflow-hidden rounded-xl border border-border bg-card">
        <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
          <span className="font-mono text-xs text-muted-foreground">{activeSql}</span>
          <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
            {rows.length} row{rows.length === 1 ? "" : "s"}
          </span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left">
                {columns.map((c) => (
                  <th key={c} className="px-4 py-2.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    {c}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={columns.length} className="px-4 py-10 text-center text-sm text-muted-foreground">
                    No rows match — change a row in the database and it appears here.
                  </td>
                </tr>
              ) : (
                rows.map((r) => (
                  <tr
                    key={keyOf(r)}
                    className={cn(
                      "border-b border-border/60 last:border-0",
                      flash[keyOf(r)] === "add" && "flash-add",
                      flash[keyOf(r)] === "mod" && "flash-mod",
                    )}
                  >
                    {columns.map((c) => (
                      <td key={c} className="px-4 py-2.5 font-mono text-[13px] text-foreground/90">
                        {c === "status" ? <StatusBadge value={String(r[c])} /> : String(r[c] ?? "")}
                      </td>
                    ))}
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Event log — makes the pushes visible */}
      <div className="mt-5">
        <h2 className="mb-2 flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">
          <ArrowRight className="size-3.5" /> pushes received
        </h2>
        <div className="rounded-xl border border-border bg-card p-1">
          {log.length === 0 ? (
            <p className="px-3 py-3 text-xs text-muted-foreground">Waiting for the first frame…</p>
          ) : (
            <ul className="divide-y divide-border/60">
              {log.map((e) => (
                <li key={e.seq} className="flex items-center gap-3 px-3 py-2 font-mono text-xs">
                  <span className="text-muted-foreground/70">{e.time}</span>
                  <span
                    className={cn(
                      "rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase",
                      e.kind === "data" && "bg-primary/10 text-primary",
                      e.kind === "diff" && "bg-warning/10 text-warning",
                      e.kind === "error" && "bg-destructive/10 text-destructive",
                    )}
                  >
                    {e.kind}
                  </span>
                  <span className="text-foreground/80">{e.text}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </main>
  );
}

function StatusBadge({ value }: { value: string }) {
  const tone =
    value === "succeeded"
      ? "bg-success/10 text-success ring-success/20"
      : value === "failed"
        ? "bg-destructive/10 text-destructive ring-destructive/20"
        : "bg-warning/10 text-warning ring-warning/20";
  return <span className={cn("rounded-full px-2 py-0.5 text-[11px] font-medium ring-1 ring-inset", tone)}>{value}</span>;
}
