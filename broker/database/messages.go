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

package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	dbtypes "github.com/srschreiber/nito/broker/database/types"
	wstypes "github.com/srschreiber/nito/websocket_types"
)

// GetUserSentMessageCount returns the number of messages the given user (by UUID) has sent in the given room.
func GetUserSentMessageCount(ctx context.Context, conn Conn, roomID, userID string) (int, error) {
	var count int
	err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM room_messages WHERE room_id = $1::uuid AND sender_user_id = $2::uuid`,
		roomID, userID,
	).Scan(&count)
	return count, err
}

// InsertRoomMessage inserts a message into the database, returning an error if the room doesn't exist or the sender isn't a joined member.
// todo: DB functions should use db structs and not types from other packages
func InsertRoomMessage(ctx context.Context, conn Conn, payload wstypes.RoomMessagePayload) error {
	const sql = `
	INSERT INTO room_messages (room_id, key_version_num, sender_message_count, sender_user_id, encrypted_text, message_type)
	SELECT $1::uuid, $2, $3, id, $5, $6
	FROM users
	WHERE username = $4
	`
	msgType := payload.MessageType
	if msgType == "" {
		msgType = wstypes.MessageTypeText
	}
	_, err := conn.Exec(ctx, sql,
		payload.RoomID, payload.RoomKeyVersion, payload.SenderMessageCount, payload.FromUsername, payload.EncryptedText, msgType,
	)
	return err
}

// GetUserDecryptableRoomMessages returns messages in the given room that the given user should be able to decrypt.
// If limit is non-nil, the most recent limit messages are returned ordered ascending; otherwise all messages are returned.
// LIMIT NULL is valid PostgreSQL and returns all rows, so limit=nil and limit=(*int)(nil) both mean no limit.
func GetUserDecryptableRoomMessages(ctx context.Context, conn Conn, roomID, userID string, limit *int) ([]dbtypes.DBRoomMessage, error) {
	// When limit is set we fetch the newest N rows first (DESC), then re-sort ascending for the caller.
	// LIMIT NULL returns all rows, so we can always pass the parameter.
	const sql = `
	WITH user_keys AS (
		SELECT DISTINCT room_key_version_num
		FROM user_room_keys
		WHERE room_id = $1::uuid AND user_id = $2::uuid
	),
	limited AS (
		SELECT rm.room_id,
			   rm.key_version_num,
			   rm.sender_message_count,
			   rm.sender_user_id,
			   sender.username,
			   rm.encrypted_text,
			   rm.message_type,
			   rm.created_at
		FROM room_messages rm
		JOIN user_keys uk ON uk.room_key_version_num = rm.key_version_num
		JOIN users sender ON sender.id = rm.sender_user_id
		WHERE rm.room_id = $1::uuid
		ORDER BY rm.created_at DESC, rm.sender_user_id ASC
		LIMIT $3
	)
	SELECT * FROM limited ORDER BY created_at ASC, sender_user_id ASC
	`
	rows, err := conn.Query(ctx, sql, roomID, userID, limit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	messages, err := ScanRows[dbtypes.DBRoomMessage](rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}
	return messages, nil
}
