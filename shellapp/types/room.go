// Copyright 2026 Sam Schreiber
//
// This file is part of nito.
//
// nito is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// nito is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with nito. If not, see <https://www.gnu.org/licenses/>.

package types

import apitypes "github.com/srschreiber/nito/api_types"

// RoomsUpdatedMsg is broadcast whenever the room list should be refreshed.
type RoomsUpdatedMsg struct {
	Rooms []apitypes.RoomEntry
}

// RoomsFetchMsg triggers an immediate room list fetch in RoomsComponent.
type RoomsFetchMsg struct{}

// RoomSelectedMsg is emitted when a room is selected (via UI or command).
type RoomSelectedMsg struct {
	RoomID string
}

// RoomMembersUpdatedMsg is broadcast when the member list for the selected room is refreshed.
type RoomMembersUpdatedMsg struct {
	Members []apitypes.RoomMemberEntry
}

// RoomMembersFetchMsg triggers an immediate members fetch.
type RoomMembersFetchMsg struct {
	RoomID string
}

type ErrorMsg struct {
	Message string
}
