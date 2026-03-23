package api_types

// User

type RegisterRequest struct {
	Username  string `json:"username" validate:"required"`
	PublicKey string `json:"publicKey" validate:"required"`
}

type RegisterResponse struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	AlreadyRegistered bool   `json:"alreadyRegistered,omitempty"`
}

// Ping

type PingRequest struct {
	Message string `json:"message" validate:"required,max=256"`
}

type PingResponse struct {
	Message string `json:"message"`
}

// Rooms

type CreateRoomRequest struct {
	Name             string `json:"name" validate:"required"`
	UserID           string `json:"userId" validate:"required"`
	EncryptedRoomKey string `json:"encryptedRoomKey" validate:"required"`
}

type CreateRoomResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RoomEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsOwner bool   `json:"isOwner"`
}

type ListRoomsResponse struct {
	Rooms []RoomEntry `json:"rooms"`
}

type InviteUserRequest struct {
	RoomID           string `json:"roomId" validate:"required"`
	InvitedUsername  string `json:"invitedUsername" validate:"required"`
	EncryptedRoomKey string `json:"encryptedRoomKey" validate:"required"`
}

type InviteUserResponse struct {
	RoomID string `json:"roomId"`
	UserID string `json:"userId"`
}

type RoomMemberEntry struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Online   bool   `json:"online"`
}

type ListRoomMembersResponse struct {
	Members []RoomMemberEntry `json:"members"`
}

type PendingInvite struct {
	RoomID   string `json:"roomId"`
	RoomName string `json:"roomName"`
}

type ListPendingInvitesResponse struct {
	Invites []PendingInvite `json:"invites"`
}

type AcceptInviteRequest struct {
	RoomID string `json:"roomId" validate:"required"`
}

type GetRoomKeyResponse struct {
	EncryptedRoomKey string `json:"encryptedRoomKey"`
	KeyVersion       int    `json:"keyVersion"`
}

type GetUserPublicKeyResponse struct {
	PublicKey string `json:"publicKey"`
}
