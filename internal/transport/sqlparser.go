package transport

// SQLParser extracts the tables a query reads. The subscribe path calls it at the
// edge and passes the result into the manager, which files the subscription in
// its matchmaker index under those tables. Owned here (consumer-defined) so the
// parser package satisfies it structurally — transport never imports the parser.
type SQLParser interface {
	Tables(sql string) ([]string, error)
}
