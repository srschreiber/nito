package types

// RegisterRequest is the payload for POST /api/v0/register.
// The public key is loaded automatically by the shellapp from the local key file.
type RegisterRequest struct {
	Username  string `json:"username" validate:"required"`
	PublicKey string `json:"publicKey" validate:"required"`
}

// RegisterResponse is returned by POST /api/v0/register.
type RegisterResponse struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	AlreadyRegistered bool   `json:"alreadyRegistered,omitempty"`
}
