package types

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
}

type GetUserPublicKeyResponse struct {
	PublicKey string `json:"publicKey"`
}
