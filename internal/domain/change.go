package domain

// ChangeOp is the kind of row change carried by a ChangeEvent.
type ChangeOp string

const (
	OpInsert ChangeOp = "insert"
	OpUpdate ChangeOp = "update"
	OpDelete ChangeOp = "delete"
)

// ChangeEvent is one committed row change, decoded from Postgres's logical
// replication stream — the Watcher's output and the change-flow's input. Old is
// the row before the change, New the row after:
//
//	insert -> New set, Old nil
//	update -> both set (Old may be primary-key-only unless the table is
//	          REPLICA IDENTITY FULL)
//	delete -> Old set, New nil
type ChangeEvent struct {
	Table string
	Op    ChangeOp
	Old   Row
	New   Row
}
