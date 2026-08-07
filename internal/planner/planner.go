package planner

import (
	"strconv"

	"github.com/Emmanuel-MacAnThony/current/internal/plan"
)

// Compile turns a query's WHERE into a Rule the filter operator can evaluate in
// memory. ok=false means we couldn't compile it — a shape we don't handle (a
// function, a subquery, an operator we don't support) — and the caller falls
// back to re-eval. A query with no WHERE compiles to an always-true rule.
//
// It walks the parse tree conservatively: the moment it meets a node it can't
// translate exactly, it gives up on the whole thing rather than guess. Guessing
// would mean silently wrong rows on screen; giving up just means a slower path.
func Compile(sql string) (plan.Rule, bool) {
	sel, err := parseSelect(sql)
	if err != nil || sel == nil {
		return nil, false
	}
	// A Filter can only maintain a plain single-table `SELECT *` — it adds/removes
	// whole rows. Anything that reshapes the result (aggregate, group, distinct,
	// limit, join, projection) can't be maintained by row add/remove, so it falls
	// back to re-eval. This gate is what keeps the fast path from ever being wrong.
	if !simpleFilterShape(sel) {
		return nil, false
	}
	where, ok := sel["whereClause"].(map[string]any)
	if !ok {
		return plan.True{}, true // no WHERE → every row matches
	}
	return compileNode(where)
}

// simpleFilterShape reports whether the SELECT is one a Filter can maintain:
// `SELECT * FROM <one table> [WHERE …] [ORDER BY …]`, with none of the clauses
// that reshape the result. ORDER BY (without LIMIT) is fine — row order doesn't
// change which rows are in the set, and the diff is by key.
func simpleFilterShape(sel map[string]any) bool {
	// Any clause that changes the result's shape disqualifies the filter.
	for _, clause := range []string{"groupClause", "havingClause", "distinctClause", "limitCount", "limitOffset", "withClause"} {
		if _, present := sel[clause]; present {
			return false
		}
	}
	// Set operations (UNION / INTERSECT / EXCEPT).
	if op, _ := sel["op"].(string); op != "" && op != "SETOP_NONE" {
		return false
	}
	// Exactly one FROM entry, and it must be a plain table (not a join or subquery).
	from, _ := sel["fromClause"].([]any)
	if len(from) != 1 {
		return false
	}
	if _, ok := from[0].(map[string]any)["RangeVar"]; !ok {
		return false
	}
	// The projection must be a single `*` — so an added row is exactly a table row,
	// with no computed or dropped columns. (Projected column lists → re-eval.)
	targets, _ := sel["targetList"].([]any)
	return len(targets) == 1 && isStar(targets[0])
}

// isStar reports whether a target-list entry is a bare `*`.
func isStar(target any) bool {
	rt, _ := target.(map[string]any)["ResTarget"].(map[string]any)
	val, _ := rt["val"].(map[string]any)
	cr, _ := val["ColumnRef"].(map[string]any)
	fields, _ := cr["fields"].([]any)
	if len(fields) != 1 {
		return false
	}
	_, star := fields[0].(map[string]any)["A_Star"]
	return star
}

// compileNode dispatches on the node kind: a comparison, an AND/OR, or give up.
func compileNode(node map[string]any) (plan.Rule, bool) {
	if expr, ok := node["A_Expr"].(map[string]any); ok {
		return compileCmp(expr)
	}
	if b, ok := node["BoolExpr"].(map[string]any); ok {
		return compileBool(b)
	}
	return nil, false // SubLink, NullTest, anything else → give up
}

// compileCmp handles a plain "column <op> constant" comparison.
func compileCmp(expr map[string]any) (plan.Rule, bool) {
	if expr["kind"] != "AEXPR_OP" {
		return nil, false // AEXPR_IN, AEXPR_LIKE, etc. → give up for now
	}
	op, ok := operatorName(expr)
	if !ok {
		return nil, false
	}
	col, ok := columnName(expr["lexpr"])
	if !ok {
		return nil, false // left side isn't a plain column (function, col-vs-col, …)
	}
	val, ok := constValue(expr["rexpr"])
	if !ok {
		return nil, false // right side isn't a constant
	}
	return plan.Cmp{Column: col, Op: op, Value: val}, true
}

// compileBool handles AND / OR of sub-conditions; any sub that won't compile
// sinks the whole rule.
func compileBool(b map[string]any) (plan.Rule, bool) {
	args, _ := b["args"].([]any)
	rules := make([]plan.Rule, 0, len(args))
	for _, a := range args {
		node, _ := a.(map[string]any)
		r, ok := compileNode(node)
		if !ok {
			return nil, false
		}
		rules = append(rules, r)
	}
	switch b["boolop"] {
	case "AND_EXPR":
		return plan.And{Rules: rules}, true
	case "OR_EXPR":
		return plan.Or{Rules: rules}, true
	}
	return nil, false // NOT_EXPR → give up for now
}

// operatorName pulls the operator (e.g. "=") out of an A_Expr, accepting only the
// comparison operators our evaluator implements exactly.
func operatorName(expr map[string]any) (string, bool) {
	names, _ := expr["name"].([]any)
	if len(names) != 1 {
		return "", false
	}
	s, _ := names[0].(map[string]any)["String"].(map[string]any)
	sval, _ := s["sval"].(string)
	switch sval {
	case "=", "<>", "!=", ">", "<", ">=", "<=":
		return sval, true
	}
	return "", false
}

// columnName pulls a bare column name out of a ColumnRef. A qualified name
// (table.col → 2 fields) or anything that isn't a ColumnRef gives up.
func columnName(node any) (string, bool) {
	m, _ := node.(map[string]any)
	cr, ok := m["ColumnRef"].(map[string]any)
	if !ok {
		return "", false
	}
	fields, _ := cr["fields"].([]any)
	if len(fields) != 1 {
		return "", false
	}
	s, _ := fields[0].(map[string]any)["String"].(map[string]any)
	sval, ok := s["sval"].(string)
	return sval, ok
}

// constValue pulls a literal out of an A_Const: string, integer, float, or bool.
// NULL and anything else give up.
func constValue(node any) (any, bool) {
	m, _ := node.(map[string]any)
	c, ok := m["A_Const"].(map[string]any)
	if !ok {
		return nil, false
	}
	if isnull, _ := c["isnull"].(bool); isnull {
		return nil, false
	}
	if s, ok := c["sval"].(map[string]any); ok {
		str, _ := s["sval"].(string)
		return str, true
	}
	if iv, ok := c["ival"].(map[string]any); ok {
		n, _ := iv["ival"].(float64) // JSON number; absent means 0
		return n, true
	}
	if fv, ok := c["fval"].(map[string]any); ok {
		f, err := strconv.ParseFloat(fmtString(fv["fval"]), 64)
		if err != nil {
			return nil, false
		}
		return f, true
	}
	if bv, ok := c["boolval"].(map[string]any); ok {
		b, _ := bv["boolval"].(bool)
		return b, true
	}
	return nil, false
}

func fmtString(v any) string {
	s, _ := v.(string)
	return s
}
