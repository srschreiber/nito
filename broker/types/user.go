package types

// RegisterRequest is the payload for POST /api/v0/register.
// TODO: In the future, registration will also require a public key. The connect
// handshake will include a (userID, publicKey) pair verified via HMAC so the
// broker can confirm the client's identity without a password. A separate signup
// endpoint will let users submit their userID, public key, and basic profile info
// which the broker will persist in Postgres. For now we keep everything in memory.
type RegisterRequest struct {
	UserID string `json:"userId" validate:"required"`
}
