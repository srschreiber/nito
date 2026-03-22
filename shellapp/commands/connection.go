package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/keys"
)

func registerCmd(args []Argument) (string, error) {
	brokerURL := extractArg(args, "b", "broker")
	if brokerURL == "" {
		return "", errors.New("register: -b/--broker <url> is required")
	}
	username := extractArg(args, "u", "user")
	if username == "" {
		return "", errors.New("register: -u/--user <username> is required")
	}

	publicKey, err := keys.LoadOrGenerate()
	if err != nil {
		return "", fmt.Errorf("register: key setup failed: %w", err)
	}

	resp, err := connection.Register(brokerURL, username, publicKey)
	if err != nil {
		return "", err
	}

	if resp.AlreadyRegistered {
		return fmt.Sprintf("user %q already registered (id: %s)", username, resp.ID), nil
	}
	return fmt.Sprintf("registered %q successfully (id: %s)", username, resp.ID), nil
}

func connectCmd(args []Argument) (string, error) {
	brokerURL := extractArg(args, "b", "broker")
	if brokerURL == "" {
		return "", errors.New("connect: -b/--broker <url> is required")
	}
	userID := extractArg(args, "u", "user")
	if userID == "" {
		return "", errors.New("connect: -u/--user <id> is required")
	}
	if !keys.HaveKeys() {
		return "", errors.New("connect: no keys found — run register first")
	}

	if err := connection.Connect(brokerURL, userID); err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}

	return fmt.Sprintf("connected to %s as %q", connection.BrokerURL(), userID), nil
}

func ping(args []Argument) (string, error) {
	brokerURL := extractArg(args, "b", "broker")
	if brokerURL == "" {
		return "", errors.New("ping: -b/--broker <url> is required")
	}

	// Normalize: strip any scheme prefix, then prepend ws://
	brokerURL = strings.TrimPrefix(brokerURL, "ws://")
	brokerURL = strings.TrimPrefix(brokerURL, "wss://")
	brokerURL = strings.TrimPrefix(brokerURL, "http://")
	brokerURL = strings.TrimPrefix(brokerURL, "https://")
	wsURL := "ws://" + brokerURL + "/ws/ping"

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return "", fmt.Errorf("ping: failed to connect to %s: %w", wsURL, err)
	}
	defer conn.Close()

	payload, _ := json.Marshal(map[string]string{"message": "ping"})
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return "", fmt.Errorf("ping: write error: %w", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return "", fmt.Errorf("ping: read error: %w", err)
	}

	return fmt.Sprintf("pong from %s: %s", brokerURL, string(msg)), nil
}
