package domain

// Row is one record from a query result: column name -> value. A map keeps it
// JSON-friendly (it marshals straight to the wire) and lets the diff match rows
// by key later without needing a fixed schema.
type Row map[string]any

// ResultSet is a query's rows in order — the whole answer a subscription
// currently shows (its Memory). Diffing keys arrive with the change-flow.
type ResultSet []Row
