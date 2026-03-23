package database

import (
	"context"

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
func InsertRoomMessage(ctx context.Context, conn Conn, payload wstypes.RoomMessagePayload) error {
	const sql = `
	INSERT INTO room_messages (room_id, key_version_num, sender_message_count, sender_user_id, encrypted_text)
	SELECT $1::uuid, $2, $3, id, $5
	FROM users
	WHERE username = $4
	`
	_, err := conn.Exec(ctx, sql,
		payload.RoomID, payload.RoomKeyVersion, payload.SenderMessageCount, payload.FromUsername, payload.EncryptedText,
	)
	if err != nil {
		return err
	}

	return nil
}
