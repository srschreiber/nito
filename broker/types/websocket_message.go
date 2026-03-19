package types

import "encoding/json"

type WebsocketMessage struct {
	RPCName   string          `json:"rpcName" validate:"required"`
	RequestID string          `json:"requestId,omitempty" validate:"required"`
	UserID    string          `json:"userId" validate:"required"`
	Nonce     string          `json:"nonce" validate:"required"`
	Timestamp int64           `json:"timestamp" validate:"required"`
	Signature string          `json:"signature" validate:"required"`
	Payload   json.RawMessage `json:"payload" validate:"required"`
}
