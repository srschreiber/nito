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
