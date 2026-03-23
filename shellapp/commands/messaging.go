package commands

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/keys"
	"github.com/srschreiber/nito/utils"
	wstypes "github.com/srschreiber/nito/websocket_types"
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

func sayCmd(args []Argument) (string, error) {
	s := connection.CurrentSession()
	if s == nil {
		return "", errors.New("say: not connected (use connect first)")
	}
	roomID := utils.DerefOrZero(connection.GetSessionRoomID())
	if roomID == "" {
		return "", errors.New("say: no room selected (use room-select first)")
	}
	text := strings.Join(extractArgValues(args, "m", "message"), " ")
	if text == "" {
		return "", errors.New("say: -m/--message <text> is required")
	}

	encRoomKey := connection.GetSessionEncryptedRoomKey()
	if encRoomKey == nil {
		return "", errors.New("say: no room key available for selected room")
	}

	roomKey, err := keys.DecryptRoomKey(utils.DerefOrZero(encRoomKey))
	if err != nil {
		return "", fmt.Errorf("say: decrypt room key: %w", err)
	}
	// TODO: Pass per-user message count to enable ratcheting. Currently always nil,
	// which reuses the same base key for every message. To add forward secrecy,
	// maintain a per-room send counter and pass it here, incrementing after each send.
	ciphertext, err := keys.EncryptMessageWithRoomKey([]byte(text), s.UserID, roomKey, nil)
	if err != nil {
		return "", fmt.Errorf("say: encrypt: %w", err)
	}

	payload, err := json.Marshal(wstypes.RoomMessagePayload{
		RoomID:        roomID,
		FromUsername:  s.UserID,
		EncryptedText: base64.StdEncoding.EncodeToString(ciphertext),
	})
	if err != nil {
		return "", fmt.Errorf("say: %w", err)
	}
	sig, err := keys.Sign(s.UserID + ":" + wstypes.RPCRoomMessage)
	if err != nil {
		return "", fmt.Errorf("say: sign: %w", err)
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
		return "", fmt.Errorf("say: %w", err)
	}
	if err := connection.Send(data); err != nil {
		return "", fmt.Errorf("say: send: %w", err)
	}
	return "", nil
}
