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

	"github.com/srschreiber/nito/broker/database"
	apitypes "github.com/srschreiber/nito/shared/api_types"
)

func (h *Handler) GetRoomMessages(w http.ResponseWriter, r *http.Request, req apitypes.GetRoomMessagesRequest) {
	username := r.Header.Get("X-Username")

	userID, err := database.GetUserIDByUsername(h.ctx, h.pool, username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	messages, err := database.GetUserDecryptableRoomMessages(h.ctx, h.pool, req.RoomID, userID, req.Limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	roomKeys, err := database.GetAllUserRoomKeys(h.ctx, h.pool, userID, req.RoomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := apitypes.GetRoomMessagesResponse{
		RoomID:       req.RoomID,
		RoomKeys:     []apitypes.RoomKey{},
		UserMessages: []apitypes.UserMessage{},
	}

	for _, key := range roomKeys {
		resp.RoomKeys = append(resp.RoomKeys, apitypes.RoomKey{
			RoomID:           key.RoomID,
			EncryptedRoomKey: key.EncryptedRoomKey,
			KeyVersion:       key.RoomKeyVersionNum,
		})
	}

	for _, message := range messages {
		resp.UserMessages = append(resp.UserMessages, apitypes.UserMessage{
			RoomID:             message.RoomID,
			RoomKeyVersion:     message.KeyVersionNum,
			EncryptedMessage:   message.EncryptedText,
			SenderMessageCount: message.SenderMessageCount,
			SenderUserID:       message.SenderUserID,
			SenderUsername:     message.SenderUserName,
			MessageType:        message.MessageType,
			CreatedAt:          message.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
