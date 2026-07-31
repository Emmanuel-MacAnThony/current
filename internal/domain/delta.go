package domain

// Delta is a change to a query's result set — what the engine pushes to a client
// so it can patch its view in place instead of re-rendering everything. All three
// carry full rows; the client matches them to its list by the subscription's key
// column. Removed carries the row that left (the client reads its key from it).
type Delta struct {
	Added    []Row
	Removed  []Row
	Modified []Row
}

// IsEmpty reports whether nothing changed — so the engine can skip pushing.
func (d Delta) IsEmpty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Modified) == 0
}
