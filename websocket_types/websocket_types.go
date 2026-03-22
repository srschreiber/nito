package websocket_types

import "encoding/json"

type ToBrokerWsMessage struct {
	RPCName   string          `json:"rpcName" validate:"required"`
	RequestID string          `json:"requestId,omitempty" validate:"required"`
	UserID    string          `json:"userId" validate:"required"`
	Nonce     string          `json:"nonce" validate:"required"`
	Timestamp int64           `json:"timestamp" validate:"required"`
	Signature string          `json:"signature" validate:"required"`
	Payload   json.RawMessage `json:"payload" validate:"required"`
}

type ToClientWsMessage struct {
	RPCName   string          `json:"rpcName" validate:"required"`
	RequestID string          `json:"requestId,omitempty" validate:"required"`
	UserID    string          `json:"userId" validate:"required"`
	Nonce     string          `json:"nonce" validate:"required"`
	Timestamp int64           `json:"timestamp" validate:"required"`
	Payload   json.RawMessage `json:"payload" validate:"required"`
}

const (
	RPCEcho           = "echo"
	RPCRoomMessage    = "room_message"
	RPCNotification   = "notification"
	RPCMembersUpdated = "members_updated"
)

const EchoMaxChars = 1024

type EchoPayload struct {
	Text string `json:"text"`
}

//create table if not exists room_message (
//room_id UUID REFERENCES rooms(id) ON DELETE CASCADE,
//key_version_num INTEGER NOT NULL,
//sender_message_count INTEGER NOT NULL DEFAULT 0, -- this is given by the client
//sender_user_id UUID NOT NULL, -- no foreign key because if user is deleted, we still want to keep the message
//encrypted_text TEXT NOT NULL,
//created_at TIMESTAMPTZ DEFAULT now(),
//updated_at TIMESTAMPTZ DEFAULT now(),
//PRIMARY KEY (room_id, sender_user_id, key_version_num, sender_message_count)
//);

type RoomMessagePayload struct {
	RoomID             string `json:"roomId" validate:"required"`
	RoomKeyVersion     int    `json:"roomKeyVersion" validate:"required" description:"the version, or epoch, of the room key used to encrypt this message"`
	SenderMessageCount int    `json:"senderMessageCount" validate:"required" description:"a strictly increasing count of messages sent by this user in this room for encryption key generation purposes."`
	FromUserID         string `json:"fromUserId" validate:"required"`
	EncryptedText      string `json:"encryptedText" validate:"required"`
}

type NotificationPayload struct {
	Text string `json:"text"`
}
