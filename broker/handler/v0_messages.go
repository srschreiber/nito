package handler

import (
	"encoding/json"
	"net/http"

	apitypes "github.com/srschreiber/nito/api_types"
	"github.com/srschreiber/nito/broker/database"
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
