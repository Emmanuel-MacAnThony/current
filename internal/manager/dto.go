package manager

// SubscribeInput is the use-case contract for a subscribe — NOT the wire message.
// The browser sends only {id, sql} and never its ClientID; the handler derives
// ClientID from the socket and assembles this Input. So the Input carries
// everything the use case needs, put together at the edge.
type SubscribeInput struct {
	ClientID string
	ID       string // subID, client-chosen
	SQL      string
}

// SubscribeOutput is empty for now: success is simply "no error." It grows to
// carry the initial rows once the query-run slice lands.
type SubscribeOutput struct{}

// UnsubscribeInput mirrors that shape: ClientID (from the socket) + the subID.
type UnsubscribeInput struct {
	ClientID string
	ID       string // subID to remove
}

type UnsubscribeOutput struct{}
