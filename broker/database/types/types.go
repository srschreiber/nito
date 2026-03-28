package types

import "time"

type User struct {
	ID        string
	Username  string
	PublicKey *string
	UpdatedAt time.Time
	CreatedAt time.Time
}

type Permission struct {
	ID          string
	Name        string
	Description *string
}

type RolePermission struct {
	Role         string
	PermissionID string
	UpdatedAt    time.Time
	CreatedAt    time.Time
}

type Room struct {
	ID              string
	Name            string
	CreatedByUserID *string
	UpdatedAt       time.Time
	CreatedAt       time.Time
}

type RoomMember struct {
	RoomID          string
	UserID          string
	InvitedByUserID *string
	JoinedAt        *time.Time
	UpdatedAt       time.Time
	CreatedAt       time.Time
}

type UserRoomRole struct {
	UserID    string
	RoomID    string
	Role      string
	UpdatedAt time.Time
	CreatedAt time.Time
}

type RoomInvite struct {
	RoomID          string
	InvitedUserID   string
	InvitedByUserID *string
	ExpiresAt       *time.Time
	UpdatedAt       time.Time
	CreatedAt       time.Time
}

type RoomKeyVersion struct {
	VersionNum        int
	RoomID            string
	GeneratedByUserID *string
	UpdatedAt         time.Time
	CreatedAt         time.Time
}

type UserRoomKey struct {
	UserID            string
	RoomID            string
	RoomKeyVersionNum int
	EncryptedRoomKey  string
	UpdatedAt         time.Time
	CreatedAt         time.Time
}

type DBRoomMessage struct {
	RoomID             string
	KeyVersionNum      int
	SenderMessageCount int
	SenderUserID       string
	SenderUserName     string
	EncryptedText      string
	MessageType        string
	CreatedAt          time.Time // RFC3339 format
}
