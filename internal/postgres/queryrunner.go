package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
)

// QueryRunner runs SQL against Postgres and returns the rows as a generic
// domain.ResultSet. It's the real implementation of the transport's QueryRunner
// port — Go's structural typing means this package never imports transport; it
// just returns the shared domain type, and main wires them together.
type QueryRunner struct {
	pool *pgxpool.Pool
}

func NewQueryRunner(pool *pgxpool.Pool) *QueryRunner {
	return &QueryRunner{pool: pool}
}

// Run executes the query and maps each row to a column->value map. It bounds the
// query with a timeout so one pathological query can't hang a subscription
// forever. Callers invoke this at the edge — never while holding the manager lock.
func (r *QueryRunner) Run(sql string) (domain.ResultSet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	var out domain.ResultSet
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(domain.Row, len(fields))
		for i, f := range fields {
			row[f.Name] = vals[i]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
