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

package handler

import (
	"encoding/json"
	"net/http"

	apitypes "github.com/srschreiber/nito/api_types"
)

func (h *Handler) createRoom(w http.ResponseWriter, r *http.Request, req apitypes.CreateRoomRequest) {
	userID := h.broker.LookupUserIDByUsername(r.Context(), req.UserID)
	if userID == "" {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	resp, err := h.broker.BrokerCreateRoom(r.Context(), userID, req.Name, req.EncryptedRoomKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) listRooms(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("user_id")
	if username == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}
	userID := h.broker.LookupUserIDByUsername(r.Context(), username)
	if userID == "" {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	rooms, err := h.broker.BrokerListUserRooms(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apitypes.ListRoomsResponse{Rooms: rooms})
}

func (h *Handler) inviteUser(w http.ResponseWriter, r *http.Request, req apitypes.InviteUserRequest) {
	username := r.Header.Get("X-Username")
	inviterID := h.broker.LookupUserIDByUsername(r.Context(), username)
	if inviterID == "" {
		http.Error(w, "inviter not found", http.StatusNotFound)
		return
	}
	resp, err := h.broker.BrokerInviteUser(r.Context(), req.RoomID, inviterID, req.InvitedUsername, req.EncryptedRoomKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) listRoomMembers(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "missing room_id", http.StatusBadRequest)
		return
	}
	members, err := h.broker.BrokerListRoomMembers(r.Context(), roomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apitypes.ListRoomMembersResponse{Members: members})
}

func (h *Handler) getRoomKey(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("X-Username")
	userID := h.broker.LookupUserIDByUsername(r.Context(), username)
	if userID == "" {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "missing room_id", http.StatusBadRequest)
		return
	}
	key, v, err := h.broker.BrokerGetRoomKey(r.Context(), userID, roomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apitypes.GetRoomKeyResponse{EncryptedRoomKey: key, KeyVersion: v})
}

func (h *Handler) listPendingInvites(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("user_id")
	if username == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}
	userID := h.broker.LookupUserIDByUsername(r.Context(), username)
	if userID == "" {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	invites, err := h.broker.BrokerListPendingInvites(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apitypes.ListPendingInvitesResponse{Invites: invites})
}

func (h *Handler) getRoomInfo(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("X-Username")
	userID := h.broker.LookupUserIDByUsername(r.Context(), username)
	if userID == "" {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "missing room_id", http.StatusBadRequest)
		return
	}
	info, err := h.broker.BrokerGetRoomInfo(r.Context(), userID, roomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (h *Handler) acceptInvite(w http.ResponseWriter, r *http.Request, req apitypes.AcceptInviteRequest) {
	username := r.Header.Get("X-Username")
	userID := h.broker.LookupUserIDByUsername(r.Context(), username)
	if userID == "" {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err := h.broker.BrokerAcceptInvite(r.Context(), userID, req.RoomID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
