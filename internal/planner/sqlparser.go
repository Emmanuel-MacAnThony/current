package planner

import (
	"encoding/json"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// parseSelect parses the SQL and returns its SelectStmt node (the generic tree
// the planner walks), or nil when the statement isn't a plain SELECT. err is
// non-nil only when the SQL doesn't parse at all.
func parseSelect(sql string) (map[string]any, error) {
	j, err := pg_query.ParseToJSON(sql)
	if err != nil {
		return nil, err
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(j), &root); err != nil {
		return nil, err
	}

	stmts, _ := root["stmts"].([]any)
	if len(stmts) == 0 {
		return nil, nil
	}
	stmt, _ := stmts[0].(map[string]any)["stmt"].(map[string]any)
	sel, _ := stmt["SelectStmt"].(map[string]any)
	return sel, nil // nil if it's not a SELECT
}
