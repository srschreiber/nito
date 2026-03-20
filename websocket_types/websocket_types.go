package websocket_types

import "encoding/json"

type IncomingWebsocketMessage struct {
	RPCName   string          `json:"rpcName" validate:"required"`
	RequestID string          `json:"requestId,omitempty" validate:"required"`
	UserID    string          `json:"userId" validate:"required"`
	Nonce     string          `json:"nonce" validate:"required"`
	Timestamp int64           `json:"timestamp" validate:"required"`
	Signature string          `json:"signature" validate:"required"`
	Payload   json.RawMessage `json:"payload" validate:"required"`
}

type OutgoingWebsocketMessage struct {
	RPCName   string          `json:"rpcName" validate:"required"`
	RequestID string          `json:"requestId,omitempty" validate:"required"`
	UserID    string          `json:"userId" validate:"required"`
	Nonce     string          `json:"nonce" validate:"required"`
	Timestamp int64           `json:"timestamp" validate:"required"`
	Payload   json.RawMessage `json:"payload" validate:"required"`
}

const EchoMaxChars = 1024

type EchoPayload struct {
	Text string `json:"text"`
}

type RoomMessagePayload struct {
	RoomID        string `json:"roomId" validate:"required"`
	FromUserID    string `json:"fromUserId" validate:"required"`
	EncryptedText string `json:"encryptedText" validate:"required"`
}

type NotificationPayload struct {
	Text string `json:"text"`
}
