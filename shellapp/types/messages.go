package types

// ConnectionStatusMsg is broadcast after any command that may change connection state.
type ConnectionStatusMsg struct {
	Connected bool
	BrokerURL string
	UserID    string
}
