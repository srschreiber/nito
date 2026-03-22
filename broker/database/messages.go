package database

import "context"
import wstypes "github.com/srschreiber/nito/websocket_types"

// InsertRoomMessage inserts a message into the database, returning an error if the room doesn't exist or the sender isn't a joined member.
func InsertRoomMessage(ctx context.Context, conn Conn, payload wstypes.RoomMessagePayload) error {
	const sql = `
	INSERT INTO room_message (room_id, key_version_num, sender_message_count, sender_user_id, encrypted_text)
	VALUES ($1, $2, $3, $4, $5)
	`
	_, err := conn.Exec(ctx, sql,
		payload.RoomID, payload.RoomKeyVersion, payload.SenderMessageCount, payload.FromUserID, payload.EncryptedText,
	)
	if err != nil {
		return err
	}

	return nil
}
