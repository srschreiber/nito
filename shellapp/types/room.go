package types

type RoomEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsOwner bool   `json:"isOwner"`
}

// RoomsUpdatedMsg is broadcast whenever the room list should be refreshed.
type RoomsUpdatedMsg struct {
	Rooms []RoomEntry
}

// RoomsFetchMsg triggers an immediate room list fetch in RoomsComponent.
type RoomsFetchMsg struct{}

// RoomSelectedMsg is emitted when a room is selected (via UI or command).
type RoomSelectedMsg struct {
	RoomID string
}

type RoomMember struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Online   bool   `json:"online"`
}

// RoomMembersUpdatedMsg is broadcast when the member list for the selected room is refreshed.
type RoomMembersUpdatedMsg struct {
	Members []RoomMember
}

// RoomMembersFetchMsg triggers an immediate members fetch.
type RoomMembersFetchMsg struct {
	RoomID string
}

type PendingInvite struct {
	RoomID   string `json:"roomId"`
	RoomName string `json:"roomName"`
}
