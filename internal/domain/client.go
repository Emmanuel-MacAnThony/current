package domain

// Subscription is one client watching one query.
type Subscription struct {
	ID       string // client-chosen, so unsubscribe and diffs target one exact view
	ClientID string // owner — so a diff for this sub knows whose conn to push down
	SQL      string // the query being watched (the parsed Plan attaches later)
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
