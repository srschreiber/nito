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

package api_types

import "time"

type GetRoomMessagesRequest struct {
	RoomID string `json:"roomId" validate:"required"`
	Limit  *int   `json:"limit"`
}

type UserMessage struct {
	RoomID             string    `json:"roomId" validate:"required"`
	RoomKeyVersion     int       `json:"roomKeyVersion" validate:"required" description:"the version, or epoch, of the room key used to encrypt this message"`
	EncryptedMessage   string    `json:"encryptedMessage" validate:"required,max=256"`
	SenderMessageCount int       `json:"senderMessageCount" validate:"required" description:"a strictly increasing count of messages sent by this user in this room for encryption key generation purposes."`
	SenderUserID       string    `json:"senderUserId" validate:"required"`
	SenderUsername     string    `json:"senderUsername" validate:"required"`
	MessageType        string    `json:"messageType"` // "text" (default) or "image"
	CreatedAt          time.Time `json:"createdAt" validate:"required" description:"time in RFC3339 formatted date"`
}

type RoomKey struct {
	RoomID           string `json:"roomId" validate:"required"`
	EncryptedRoomKey string `json:"encryptedRoomKey" validate:"required"`
	KeyVersion       int    `json:"keyVersion" validate:"required"`
}

type GetRoomMessagesResponse struct {
	RoomID       string        `json:"roomId" validate:"required"`
	RoomKeys     []RoomKey     `json:"historicKeys" validate:"required" description:"the list of historic room keys needed to decrypt the messages in this response, ordered by keyVersion ascending"`
	UserMessages []UserMessage `json:"userMessages" validate:"required" description:"the list of messages in this room, ordered by createdAt ascending"`
}
