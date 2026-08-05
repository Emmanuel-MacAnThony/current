// Package parser extracts the set of tables a SQL query reads. The matchmaker
// indexes each subscription under these tables so a change routes only to the
// queries that actually read the changed table.
package parser

import (
	"encoding/json"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// Parser extracts table names using Postgres's own grammar (libpg_query), so it
// understands exactly what the database does — joins, subqueries, CTEs. It's a
// value type with no state; a zero Parser is ready to use.
type Parser struct{}

// Tables returns every table the query reads, de-duplicated. Names are the bare
// relation names (no schema) to match what the WAL watcher emits.
//
// It works off the parse tree as JSON and collects the relation name from every
// RangeVar node — a RangeVar is how Postgres represents "a table appearing in the
// query," wherever it appears (FROM, JOIN, subquery, CTE reference). Collecting
// all of them means we never under-count and leave a query that silently stops
// updating. (A CTE's own name also parses as a RangeVar; it becomes a harmless
// phantom table that simply never receives a change.)
func (Parser) Tables(sql string) ([]string, error) {
	tree, err := pg_query.ParseToJSON(sql)
	if err != nil {
		return nil, err
	}

	var root any
	if err := json.Unmarshal([]byte(tree), &root); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var tables []string
	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if rv, ok := n["RangeVar"].(map[string]any); ok {
				if name, ok := rv["relname"].(string); ok && name != "" {
					if _, dup := seen[name]; !dup {
						seen[name] = struct{}{}
						tables = append(tables, name)
					}
				}
			}
			for _, child := range n {
				walk(child)
			}
		case []any:
			for _, child := range n {
				walk(child)
			}
		}
	}
	walk(root)

	return tables, nil
}
