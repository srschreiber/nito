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

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/srschreiber/nito/broker/database"
	apitypes "github.com/srschreiber/nito/shared/api_types"
	wstypes "github.com/srschreiber/nito/shared/websocket_types"
)

// BrokerCreateRoom creates a room owned by userID, storing the encrypted room key.
func (b *Broker) BrokerCreateRoom(ctx context.Context, userID, name, encryptedRoomKey string) (*apitypes.CreateRoomResponse, error) {
	room, err := database.CreateRoom(ctx, b.DB, name, userID, encryptedRoomKey)
	if err != nil {
		return nil, err
	}
	return &apitypes.CreateRoomResponse{ID: room.ID, Name: room.Name}, nil
}

// BrokerListUserRooms returns all rooms the user has joined.
func (b *Broker) BrokerListUserRooms(ctx context.Context, userID string) ([]apitypes.RoomEntry, error) {
	rows, err := database.ListUserRooms(ctx, b.DB, userID)
	if err != nil {
		return nil, err
	}
	entries := make([]apitypes.RoomEntry, len(rows))
	for i, r := range rows {
		entries[i] = apitypes.RoomEntry{ID: r.RoomID, Name: r.RoomName, IsOwner: r.IsOwner}
	}
	return entries, nil
}

// BrokerInviteUser adds invitedUserID to roomID as a pending member, storing their encrypted room key.
// If the invited user is connected via WebSocket, a push notification is sent.
func (b *Broker) BrokerInviteUser(ctx context.Context, roomID, invitedByUserID, invitedUsername, encryptedRoomKey string) (*apitypes.InviteUserResponse, error) {
	invitedUserID := b.LookupUserIDByUsername(ctx, invitedUsername)
	if invitedUserID == "" {
		return nil, fmt.Errorf("user %q not found", invitedUsername)
	}
	member, err := database.InviteUserToRoom(ctx, b.DB, roomID, invitedUserID, invitedByUserID, encryptedRoomKey)
	if err != nil {
		return nil, err
	}

	var roomName string
	_ = b.DB.QueryRow(ctx, `SELECT name FROM rooms WHERE id = $1`, roomID).Scan(&roomName)
	var inviterUsername string
	_ = b.DB.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, invitedByUserID).Scan(&inviterUsername)

	text := fmt.Sprintf(
		"%s invited you to %q\n\nRun 'room-invites' to list invitations, 'room-accept -r %s' to accept.",
		inviterUsername, roomName, roomID,
	)
	b.sendNotification(invitedUserID, text)

	return &apitypes.InviteUserResponse{RoomID: member.RoomID, UserID: member.UserID}, nil
}

// notifyMembersUpdated fans out a "members_updated" RPC to every connected co-member of userID.
func (b *Broker) notifyMembersUpdated(userID string) {
	coMembers, err := database.GetCoMemberUserIDs(context.Background(), b.DB, userID)
	if err != nil {
		log.Printf("notifyMembersUpdated: query co-members for %s: %v", userID, err)
		return
	}
	payload := json.RawMessage("{}")
	for _, coID := range coMembers {
		b.sendToClient(coID, wstypes.RPCMembersUpdated, payload)
	}
}

// sendRoomMessage processes and fans out an incoming room message.
func (b *Broker) sendRoomMessage(client *Client, message wstypes.ToBrokerWsMessage) error {
	var payload wstypes.RoomMessagePayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal room_message payload: %w", err)
	}

	members, err := database.ListRoomMembers(context.Background(), b.DB, payload.RoomID)
	if err != nil {
		return err
	}

	for _, member := range members {
		if member.UserID == client.Session.UserID {
			continue
		}
		b.sendToClient(member.UserID, wstypes.RPCRoomMessage, message.Payload)
	}

	b.inflightMessages.Enqueue(payload)
	return nil
}

// sendNotification pushes a notification message to a connected user. No-op if user is offline.
func (b *Broker) sendNotification(userID, text string) {
	payload, err := json.Marshal(wstypes.NotificationPayload{Text: text})
	if err != nil {
		log.Printf("notification: marshal payload: %v", err)
		return
	}
	b.sendToClient(userID, wstypes.RPCNotification, payload)
}

// BrokerListRoomMembers returns joined members of a room with their usernames and online status.
func (b *Broker) BrokerListRoomMembers(ctx context.Context, roomID string) ([]apitypes.RoomMemberEntry, error) {
	rows, err := database.ListRoomMembersWithUsernames(ctx, b.DB, roomID)
	if err != nil {
		return nil, err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	entries := make([]apitypes.RoomMemberEntry, len(rows))
	for i, r := range rows {
		entries[i] = apitypes.RoomMemberEntry{
			UserID:   r.UserID,
			Username: r.Username,
			Online:   b.clientMap[r.UserID] != nil,
		}
	}
	return entries, nil
}

// BrokerListPendingInvites returns rooms the user has been invited to but not yet joined.
func (b *Broker) BrokerListPendingInvites(ctx context.Context, userID string) ([]apitypes.PendingInvite, error) {
	rows, err := database.ListPendingInvites(ctx, b.DB, userID)
	if err != nil {
		return nil, err
	}
	invites := make([]apitypes.PendingInvite, len(rows))
	for i, r := range rows {
		invites[i] = apitypes.PendingInvite{RoomID: r.RoomID, RoomName: r.RoomName}
	}
	return invites, nil
}

// BrokerAcceptInvite sets joined_at = now() for a pending room_members row.
func (b *Broker) BrokerAcceptInvite(ctx context.Context, userID, roomID string) error {
	return database.AcceptRoomInvite(ctx, b.DB, userID, roomID)
}

// BrokerGetRoomKey returns the user's encrypted room key for a given room.
func (b *Broker) BrokerGetRoomKey(ctx context.Context, userID, roomID string) (string, int, error) {
	key, err := database.GetUserRoomKey(ctx, b.DB, userID, roomID)
	if err != nil {
		return "", -1, err
	}
	return key.EncryptedRoomKey, key.RoomKeyVersionNum, nil
}

// BrokerGetRoomInfo returns room info for the given user in the given room.
func (b *Broker) BrokerGetRoomInfo(ctx context.Context, userID, roomID string) (*apitypes.GetRoomInfoResponse, error) {
	count, err := database.GetUserSentMessageCount(ctx, b.DB, roomID, userID)
	if err != nil {
		return nil, err
	}
	return &apitypes.GetRoomInfoResponse{SentMessageCount: count}, nil
}

// BrokerGetUserPublicKey returns the public key PEM for the given username.
func (b *Broker) BrokerGetUserPublicKey(ctx context.Context, username string) (string, error) {
	pub, err := database.GetUserPublicKeyByUsername(ctx, b.DB, username)
	if err != nil {
		return "", err
	}
	if pub == nil {
		return "", fmt.Errorf("user %q has no public key", username)
	}
	return *pub, nil
}
