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
