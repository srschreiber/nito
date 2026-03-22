package database

import "context"
import wstypes "github.com/srschreiber/nito/websocket_types"

// OutboundRoomMessages collects messages that need to be inserted into the database, allowing
// routing to user's specific channels to make sure that database insertions will not block delivery.
// since there is only one broker, this will also guarantee delivery order.
type OutboundRoomMessages struct {
}

// InsertRoomMessage inserts a message into the database, returning an error if the room doesn't exist or the sender isn't a joined member.
func InsertRoomMessage(ctx context.Context, conn Conn, payload wstypes.RoomMessagePayload) error {
	return nil
}
