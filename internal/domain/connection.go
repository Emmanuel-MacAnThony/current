package domain

// Connection is the standardized way the engine talks back to a client: send a
// frame, or close. It's a port — the api layer's websocket adapter implements
// it — so the domain and manager never import the transport, stay pure, and are
// testable with a fake.
type Connection interface {
	Send(msg []byte) error
	Close() error
}
