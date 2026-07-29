package transport

import "github.com/Emmanuel-MacAnThony/current/internal/domain"

// QueryRunner runs a subscription's SQL and returns the current rows. The
// dispatch depends on this port — not on pgx — so the query-run is testable with
// a fake, and the real Postgres adapter is wired in at the composition root. It
// runs at the edge, outside the manager's lock, so a slow query never blocks the
// in-memory state.
type QueryRunner interface {
	Run(sql string) (domain.ResultSet, error)
}
