package diff

import (
	"reflect"

	"github.com/Emmanuel-MacAnThony/current/internal/domain"
)

// ByKey computes the change from old to next, matching rows by the given key
// column. This is the re-eval path's diff: re-run a query, compare the fresh rows
// to the stored ones. Rows are matched on row[key]; the key column is assumed
// present in both (validated at subscribe time).
func ByKey(old, next domain.ResultSet, key string) domain.Delta {
	oldByKey := index(old, key)
	nextByKey := index(next, key)

	var d domain.Delta
	for k, newRow := range nextByKey {
		oldRow, existed := oldByKey[k]
		switch {
		case !existed:
			d.Added = append(d.Added, newRow) // key only in new → appeared
		case !reflect.DeepEqual(oldRow, newRow):
			d.Modified = append(d.Modified, newRow) // key in both, values differ → changed
		}
	}
	for k, oldRow := range oldByKey {
		if _, still := nextByKey[k]; !still {
			d.Removed = append(d.Removed, oldRow) // key only in old → left
		}
	}
	return d
}

// index maps rows by their key value. The key value is used as a Go map key, so
// it must be comparable — which row identities (id / primary key) always are.
func index(rows domain.ResultSet, key string) map[any]domain.Row {
	m := make(map[any]domain.Row, len(rows))
	for _, r := range rows {
		m[r[key]] = r
	}
	return m
}
