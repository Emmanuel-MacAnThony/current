package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
)

const (
	publicationName = "current_pub"
	slotName        = "current_slot"
	standbyInterval = 10 * time.Second
)

// Watcher taps a Postgres logical replication slot and emits each committed row
// change as a domain.ChangeEvent. It implements the engine's ChangeSource port;
// the engine never sees pglogrepl. All the messy replication protocol lives here.
type Watcher struct {
	dsn string
}

func NewWatcher(dsn string) *Watcher {
	return &Watcher{dsn: dsn}
}

// Run streams changes until ctx is cancelled (clean stop, nil error) or the
// stream fails. It ensures the publication + slot exist on first run.
func (w *Watcher) Run(ctx context.Context, handle func(domain.ChangeEvent)) error {
	if err := ensurePublication(ctx, w.dsn); err != nil {
		return fmt.Errorf("ensure publication: %w", err)
	}

	conn, err := pgconn.Connect(ctx, replicationDSN(w.dsn))
	if err != nil {
		return fmt.Errorf("replication connect: %w", err)
	}
	defer conn.Close(ctx)

	if err := ensureSlot(ctx, w.dsn, conn); err != nil {
		return fmt.Errorf("ensure slot: %w", err)
	}

	sysident, err := pglogrepl.IdentifySystem(ctx, conn)
	if err != nil {
		return fmt.Errorf("identify system: %w", err)
	}

	pluginArgs := []string{
		"proto_version '1'",
		fmt.Sprintf("publication_names '%s'", publicationName),
	}
	if err := pglogrepl.StartReplication(ctx, conn, slotName, sysident.XLogPos,
		pglogrepl.StartReplicationOptions{PluginArgs: pluginArgs}); err != nil {
		return fmt.Errorf("start replication: %w", err)
	}

	relations := map[uint32]*pglogrepl.RelationMessage{}
	clientXLogPos := sysident.XLogPos
	nextStandby := time.Now().Add(standbyInterval)

	for {
		if ctx.Err() != nil {
			return nil
		}
		if time.Now().After(nextStandby) {
			if err := pglogrepl.SendStandbyStatusUpdate(ctx, conn,
				pglogrepl.StandbyStatusUpdate{WALWritePosition: clientXLogPos}); err != nil {
				return fmt.Errorf("standby status update: %w", err)
			}
			nextStandby = time.Now().Add(standbyInterval)
		}

		rctx, cancel := context.WithDeadline(ctx, nextStandby)
		raw, err := conn.ReceiveMessage(rctx)
		cancel()
		if err != nil {
			if pgconn.Timeout(err) {
				continue // just the standby tick; loop back to send it
			}
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("receive: %w", err)
		}

		cd, ok := raw.(*pgproto3.CopyData)
		if !ok {
			continue
		}
		switch cd.Data[0] {
		case pglogrepl.PrimaryKeepaliveMessageByteID:
			pk, err := pglogrepl.ParsePrimaryKeepaliveMessage(cd.Data[1:])
			if err != nil {
				return fmt.Errorf("parse keepalive: %w", err)
			}
			if pk.ReplyRequested {
				nextStandby = time.Time{} // send a standby update immediately
			}
		case pglogrepl.XLogDataByteID:
			xld, err := pglogrepl.ParseXLogData(cd.Data[1:])
			if err != nil {
				return fmt.Errorf("parse xlogdata: %w", err)
			}
			decode(xld.WALData, relations, handle)
			clientXLogPos = xld.WALStart + pglogrepl.LSN(len(xld.WALData))
		}
	}
}

// decode turns one pgoutput logical-replication message into a ChangeEvent.
// Relation messages describe a table's columns (cached by id); the Insert/
// Update/Delete messages carry tuples we map to rows using that description.
func decode(walData []byte, relations map[uint32]*pglogrepl.RelationMessage, handle func(domain.ChangeEvent)) {
	msg, err := pglogrepl.Parse(walData)
	if err != nil {
		return
	}
	switch m := msg.(type) {
	case *pglogrepl.RelationMessage:
		relations[m.RelationID] = m
	case *pglogrepl.InsertMessage:
		if rel := relations[m.RelationID]; rel != nil {
			handle(domain.ChangeEvent{Table: rel.RelationName, Op: domain.OpInsert, New: tupleToRow(rel, m.Tuple)})
		}
	case *pglogrepl.UpdateMessage:
		if rel := relations[m.RelationID]; rel != nil {
			handle(domain.ChangeEvent{Table: rel.RelationName, Op: domain.OpUpdate,
				Old: tupleToRow(rel, m.OldTuple), New: tupleToRow(rel, m.NewTuple)})
		}
	case *pglogrepl.DeleteMessage:
		if rel := relations[m.RelationID]; rel != nil {
			handle(domain.ChangeEvent{Table: rel.RelationName, Op: domain.OpDelete, Old: tupleToRow(rel, m.OldTuple)})
		}
	}
}

// tupleToRow maps a pgoutput tuple to a column->value row using the relation's
// column names. pgoutput v1 sends values as text; 'u' means an unchanged TOAST
// value the server didn't resend, so we skip it.
func tupleToRow(rel *pglogrepl.RelationMessage, tuple *pglogrepl.TupleData) domain.Row {
	if tuple == nil {
		return nil
	}
	row := make(domain.Row, len(tuple.Columns))
	for i, col := range tuple.Columns {
		name := rel.Columns[i].Name
		switch col.DataType {
		case 'n':
			row[name] = nil
		case 't':
			row[name] = string(col.Data)
		}
	}
	return row
}

func ensurePublication(ctx context.Context, dsn string) error {
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer c.Close(ctx)

	var exists bool
	if err := c.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_publication WHERE pubname=$1)", publicationName).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		// FOR ALL TABLES keeps it simple; the Matchmaker filters by table anyway.
		if _, err := c.Exec(ctx, fmt.Sprintf("CREATE PUBLICATION %s FOR ALL TABLES", publicationName)); err != nil {
			return err
		}
	}
	return nil
}

func ensureSlot(ctx context.Context, dsn string, repl *pgconn.PgConn) error {
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	var exists bool
	err = c.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_replication_slots WHERE slot_name=$1)", slotName).Scan(&exists)
	c.Close(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = pglogrepl.CreateReplicationSlot(ctx, repl, slotName, "pgoutput", pglogrepl.CreateReplicationSlotOptions{})
	return err
}

// replicationDSN adds the replication=database parameter the streaming
// connection needs (a normal connection can't START_REPLICATION).
func replicationDSN(dsn string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "replication=database"
}
