package websocket

import (
	"context"

	"github.com/srschreiber/nito/broker/database"
	"github.com/srschreiber/nito/broker/types"
)

// BrokerCreateRoom creates a room owned by userID, storing the encrypted room key.
func (b *Broker) BrokerCreateRoom(ctx context.Context, userID, name, encryptedRoomKey string) (*types.CreateRoomResponse, error) {
	room, err := database.CreateRoom(ctx, b.db, name, userID, encryptedRoomKey)
	if err != nil {
		return nil, err
	}
	return &types.CreateRoomResponse{ID: room.ID, Name: room.Name}, nil
}

// BrokerListUserRooms returns all rooms the user has joined.
func (b *Broker) BrokerListUserRooms(ctx context.Context, userID string) ([]types.RoomEntry, error) {
	rows, err := database.ListUserRooms(ctx, b.db, userID)
	if err != nil {
		return nil, err
	}
	entries := make([]types.RoomEntry, len(rows))
	for i, r := range rows {
		entries[i] = types.RoomEntry{ID: r.RoomID, Name: r.RoomName, IsOwner: r.IsOwner}
	}
	return entries, nil
}
