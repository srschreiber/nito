package types

// ConnectionStatusMsg is broadcast after any command that may change connection state.
type ConnectionStatusMsg struct {
	Connected bool
	BrokerURL string
	UserID    string
}

// ConnectedMsg is sent once after a successful connect, to re-arm WS listeners.
type ConnectedMsg struct{}
