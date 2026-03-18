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
