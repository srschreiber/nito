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

package commands

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/srschreiber/nito/shared/utils"
	wstypes "github.com/srschreiber/nito/shared/websocket_types"
	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/keys"
)

func echoCmd(args []Argument) (string, error) {
	s := connection.CurrentSession()
	if s == nil {
		return "", errors.New("echo: not connected (use connect first)")
	}

	text := strings.Join(extractArgValues(args, "m", "message"), " ")
	if text == "" {
		return "", errors.New("echo: -m/--message <text> is required")
	}

	payload, err := json.Marshal(wstypes.EchoPayload{Text: text})
	if err != nil {
		return "", fmt.Errorf("echo: %w", err)
	}
	sig, err := keys.Sign(s.UserID + ":" + wstypes.RPCEcho)
	if err != nil {
		return "", fmt.Errorf("echo: sign: %w", err)
	}
	msg := wstypes.ToBrokerWsMessage{
		RPCName:   wstypes.RPCEcho,
		RequestID: fmt.Sprintf("%d", time.Now().UnixNano()),
		UserID:    s.UserID,
		Nonce:     fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Signature: sig,
		Payload:   payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("echo: %w", err)
	}
	if err := connection.Send(data); err != nil {
		return "", fmt.Errorf("echo: send: %w", err)
	}
	// Response arrives asynchronously via the readLoop → incomingChan → TUI model.
	return "", nil
}

// SendRoomMessage sends text to the currently selected room without going through the command parser.
// Used by chat mode so raw input can be sent directly.
func SendRoomMessage(text string) error {
	return sendRoomMessageWithType(text, wstypes.MessageTypeText)
}

// SendRoomImage sends an ASCII-art image string to the currently selected room,
// marking it with MessageType "image" so recipients render it without re-styling.
func SendRoomImage(text string) error {
	return sendRoomMessageWithType(text, wstypes.MessageTypeImage)
}

func sendRoomMessageWithType(text, msgType string) error {
	s := connection.CurrentSession()
	if s == nil {
		return errors.New("say: not connected (use connect first)")
	}
	roomID := utils.DerefOrZero(connection.GetSessionRoomID())
	if roomID == "" {
		return errors.New("say: no room selected (use room-select first)")
	}
	if text == "" {
		return errors.New("say: message text is required")
	}

	ukc, err := connection.GetOrCreateRoomKeyChain()
	if err != nil {
		return fmt.Errorf("say: get room key chain: %w", err)
	}
	roomKeyVersion := utils.DerefOrZero(connection.GetSessionRoomKeyVersion())

	roomInfo := connection.GetSessionRoomInfo()
	if roomInfo == nil {
		return errors.New("say: no room info available for selected room")
	}

	ciphertext, err := ukc.EncryptMessageWithRoomKey([]byte(text), s.UserID, &roomInfo.SentMessageCount)
	if err != nil {
		return fmt.Errorf("say: encrypt: %w", err)
	}

	payload, err := json.Marshal(wstypes.RoomMessagePayload{
		RoomID:             roomID,
		RoomKeyVersion:     roomKeyVersion,
		SenderMessageCount: roomInfo.SentMessageCount,
		FromUsername:       s.UserID,
		EncryptedText:      base64.StdEncoding.EncodeToString(ciphertext),
		MessageType:        msgType,
	})
	if err != nil {
		return fmt.Errorf("say: %w", err)
	}
	sig, err := keys.Sign(s.UserID + ":" + wstypes.RPCRoomMessage)
	if err != nil {
		return fmt.Errorf("say: sign: %w", err)
	}
	msg := wstypes.ToBrokerWsMessage{
		RPCName:   wstypes.RPCRoomMessage,
		RequestID: fmt.Sprintf("%d", time.Now().UnixNano()),
		UserID:    s.UserID,
		Nonce:     fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Signature: sig,
		Payload:   payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("say: %w", err)
	}
	if err := connection.Send(data); err != nil {
		return fmt.Errorf("say: send: %w", err)
	}
	connection.IncrementSessionSentMessageCount()
	return nil
}

func sayCmd(args []Argument) (string, error) {
	text := strings.Join(extractArgValues(args, "m", "message"), " ")
	if text == "" {
		return "", errors.New("say: -m/--message <text> is required")
	}
	return "", sendRoomMessageWithType(text, wstypes.MessageTypeText)
}
