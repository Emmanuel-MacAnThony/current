package manager

// The matchmaker index. `byTable` maps a table to the set of subscriptions that
// read it, so a change on a table routes to exactly its subscribers in one lookup
// instead of a scan over every subscription. A sub that reads several tables sits
// in several buckets; the inner map is keyed by subKey so it can't land in one
// bucket twice. All three helpers assume the caller holds the manager lock — the
// index moves in lockstep with the sub map it mirrors.

// subKey uniquely names a subscription across clients (subIDs are only unique per
// client), so it's safe as the set key inside a bucket.
func subKey(clientID, subID string) string {
	return clientID + "/" + subID
}

// indexAdd files a sub under each table its query reads.
func (m *Manager) indexAdd(ref SubRef, tables []string) {
	k := subKey(ref.ClientID, ref.SubID)
	for _, t := range tables {
		bucket := m.byTable[t]
		if bucket == nil {
			bucket = make(map[string]SubRef)
			m.byTable[t] = bucket
		}
		bucket[k] = ref
	}
}

// indexRemove pulls a sub out of every table bucket it was filed under, dropping
// buckets that empty out so the map doesn't accumulate dead tables.
func (m *Manager) indexRemove(clientID, subID string, tables []string) {
	k := subKey(clientID, subID)
	for _, t := range tables {
		bucket := m.byTable[t]
		if bucket == nil {
			continue
		}
		delete(bucket, k)
		if len(bucket) == 0 {
			delete(m.byTable, t)
		}
	}
}

// SubsForTable returns the subscriptions that read the given table — the routed
// set for a single change. Read-locked and cheap: it only copies references out
// of one bucket; the slow re-run of each query happens outside, in the engine.
func (m *Manager) SubsForTable(table string) []SubRef {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucket := m.byTable[table]
	refs := make([]SubRef, 0, len(bucket))
	for _, ref := range bucket {
		refs = append(refs, ref)
	}
	return refs
}
