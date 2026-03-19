package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

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

// BrokerInviteUser adds invitedUserID to roomID as a pending member, storing their encrypted room key.
// If the invited user is connected via WebSocket, a push notification is sent.
func (b *Broker) BrokerInviteUser(ctx context.Context, roomID, invitedByUserID, invitedUsername, encryptedRoomKey string) (*types.InviteUserResponse, error) {
	invitedUserID := b.LookupUserIDByUsername(ctx, invitedUsername)
	if invitedUserID == "" {
		return nil, fmt.Errorf("user %q not found", invitedUsername)
	}
	member, err := database.InviteUserToRoom(ctx, b.db, roomID, invitedUserID, invitedByUserID, encryptedRoomKey)
	if err != nil {
		return nil, err
	}

	// Best-effort: look up room name and inviter username for the notification.
	var roomName string
	_ = b.db.QueryRow(ctx, `SELECT name FROM rooms WHERE id = $1`, roomID).Scan(&roomName)
	var inviterUsername string
	_ = b.db.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, invitedByUserID).Scan(&inviterUsername)

	text := fmt.Sprintf(
		"%s invited you to %q\n\nRun 'room-invites' to list invitations, 'room-accept -r %s' to accept.",
		inviterUsername, roomName, roomID,
	)
	b.sendNotification(invitedUserID, text)

	return &types.InviteUserResponse{RoomID: member.RoomID, UserID: member.UserID}, nil
}

// notifyMembersUpdated fans out a "members_updated" RPC to every connected co-member of userID.
// Called when a user goes online or offline so room member lists can be refreshed.
func (b *Broker) notifyMembersUpdated(userID string) {
	coMembers, err := database.GetCoMemberUserIDs(context.Background(), b.db, userID)
	if err != nil {
		log.Printf("notifyMembersUpdated: query co-members for %s: %v", userID, err)
		return
	}
	msg := types.WebsocketMessage{
		RPCName:   "members_updated",
		RequestID: fmt.Sprintf("%d", time.Now().UnixNano()),
		UserID:    userID,
		Nonce:     fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Payload:   []byte("{}"),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("notifyMembersUpdated: marshal: %v", err)
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, coID := range coMembers {
		client := b.clientMap[coID]
		if client == nil {
			continue
		}
		select {
		case client.send <- data:
		default:
			log.Printf("notifyMembersUpdated: send channel full for user %s", coID)
		}
	}
}

// sendNotification pushes a notification message to a connected user. No-op if user is offline.
func (b *Broker) sendNotification(userID, text string) {
	b.mu.RLock()
	client := b.clientMap[userID]
	b.mu.RUnlock()
	if client == nil {
		return
	}
	payload, err := json.Marshal(types.NotificationPayload{Text: text})
	if err != nil {
		log.Printf("notification: marshal payload: %v", err)
		return
	}
	msg := types.WebsocketMessage{
		RPCName:   "notification",
		RequestID: fmt.Sprintf("%d", time.Now().UnixNano()),
		UserID:    userID,
		Nonce:     fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("notification: marshal message: %v", err)
		return
	}
	select {
	case client.send <- data:
	default:
		log.Printf("notification: send channel full for user %s", userID)
	}
}

// BrokerListRoomMembers returns joined members of a room with their usernames and online status.
func (b *Broker) BrokerListRoomMembers(ctx context.Context, roomID string) ([]types.RoomMemberEntry, error) {
	rows, err := database.ListRoomMembersWithUsernames(ctx, b.db, roomID)
	if err != nil {
		return nil, err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	entries := make([]types.RoomMemberEntry, len(rows))
	for i, r := range rows {
		entries[i] = types.RoomMemberEntry{
			UserID:   r.UserID,
			Username: r.Username,
			Online:   b.clientMap[r.UserID] != nil,
		}
	}
	return entries, nil
}

// BrokerListPendingInvites returns rooms the user has been invited to but not yet joined.
func (b *Broker) BrokerListPendingInvites(ctx context.Context, userID string) ([]types.PendingInvite, error) {
	rows, err := database.ListPendingInvites(ctx, b.db, userID)
	if err != nil {
		return nil, err
	}
	invites := make([]types.PendingInvite, len(rows))
	for i, r := range rows {
		invites[i] = types.PendingInvite{RoomID: r.RoomID, RoomName: r.RoomName}
	}
	return invites, nil
}

// BrokerAcceptInvite sets joined_at = now() for a pending room_members row.
func (b *Broker) BrokerAcceptInvite(ctx context.Context, userID, roomID string) error {
	return database.AcceptRoomInvite(ctx, b.db, userID, roomID)
}

// BrokerGetRoomKey returns the user's encrypted room key for a given room.
func (b *Broker) BrokerGetRoomKey(ctx context.Context, userID, roomID string) (string, error) {
	key, err := database.GetUserRoomKey(ctx, b.db, userID, roomID)
	if err != nil {
		return "", err
	}
	return key.EncryptedRoomKey, nil
}

// BrokerGetUserPublicKey returns the public key PEM for the given username.
func (b *Broker) BrokerGetUserPublicKey(ctx context.Context, username string) (string, error) {
	pub, err := database.GetUserPublicKeyByUsername(ctx, b.db, username)
	if err != nil {
		return "", err
	}
	if pub == nil {
		return "", fmt.Errorf("user %q has no public key", username)
	}
	return *pub, nil
}
