package domain

// Subscription is one client watching one query.
type Subscription struct {
	ID       string    // client-chosen, so unsubscribe and diffs target one exact view
	ClientID string    // owner — so a diff for this sub knows whose conn to push down
	SQL      string    // the recipe: re-run on change + tells the planner which tables it reads
	Key      string    // column that identifies a row, so the diff can match old↔new (default "id")
	Result   ResultSet // Memory: the rows shown now — the "before" to diff against
	Tables   []string  // tables this query reads — kept so teardown can find every index bucket it's in
	Operator Operator  // how this sub reacts to a change — a Filter (in-memory) or ReEval (re-run), chosen at subscribe
}

// Client is one connection and the subscriptions opened over it. All in-memory,
// alive only while the connection is — nothing here is persisted.
type Client struct {
	ID   string
	Conn Connection               // the live socket (the only non-data field)
	Subs map[string]*Subscription // subID -> Subscription
}

// NewClient starts a client with an empty subscription set, ready to register.
func NewClient(id string, conn Connection) *Client {
	return &Client{ID: id, Conn: conn, Subs: make(map[string]*Subscription)}
}
