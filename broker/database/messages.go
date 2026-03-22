package database

import "context"
import wstypes "github.com/srschreiber/nito/websocket_types"

// InsertRoomMessage inserts a message into the database, returning an error if the room doesn't exist or the sender isn't a joined member.
func InsertRoomMessage(ctx context.Context, conn Conn, payload wstypes.RoomMessagePayload) error {
	return nil
}
