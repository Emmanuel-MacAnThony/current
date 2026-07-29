package manager

import "errors"

// The named error states the subscribe/unsubscribe flows can end in. They're
// developer-facing: the transport turns each into an error frame the client's
// console shows, so a frontend bug (missing id/sql, reused id, unsubscribing
// something not there, acting before the connection is registered) surfaces
// loudly instead of silently corrupting state.
var (
	ErrEmptySubID     = errors.New("empty subscription id")
	ErrEmptySQL       = errors.New("empty sql")
	ErrDuplicateSub   = errors.New("already subscribed with that id")
	ErrClientNotFound = errors.New("client not registered")
	ErrSubNotFound    = errors.New("no such subscription")
)
