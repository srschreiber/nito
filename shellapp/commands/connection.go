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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/keys"
)

var pendingRegisterBroker string
var pendingRegisterUsername string

func registerCmd(args []Argument) (Signal, error) {
	brokerURL := extractArg(args, "b", "broker")
	if brokerURL == "" {
		return SignalNone, errors.New("register: -b/--broker <url> is required")
	}
	username := extractArg(args, "u", "user")
	if username == "" {
		return SignalNone, errors.New("register: -u/--user <username> is required")
	}
	pendingRegisterBroker = brokerURL
	pendingRegisterUsername = username
	return SignalNeedRegisterPassword, nil
}

// CompleteRegister finishes the register flow with the password the user entered.
func CompleteRegister(password string) (string, Signal, error) {
	broker := pendingRegisterBroker
	username := pendingRegisterUsername
	pendingRegisterBroker = ""
	pendingRegisterUsername = ""

	publicKey, err := keys.LoadOrGenerate()
	if err != nil {
		return "", SignalNone, fmt.Errorf("register: key setup failed: %w", err)
	}
	resp, err := connection.Register(broker, username, password, publicKey)
	if err != nil {
		return "", SignalNone, err
	}
	if resp.AlreadyRegistered {
		return fmt.Sprintf("user %q already registered (id: %s)", username, resp.ID), SignalNone, nil
	}
	return fmt.Sprintf("registered %q successfully (id: %s)", username, resp.ID), SignalNone, nil
}

// pendingLogin holds broker/username across the two-step login flow (args → password prompt).
var pendingLoginBroker string
var pendingLoginUsername string

// loginCmd parses login arguments and signals the TUI to prompt for a hidden password.
func loginCmd(args []Argument) (Signal, error) {
	brokerURL := extractArg(args, "b", "broker")
	if brokerURL == "" {
		return SignalNone, errors.New("login: -b/--broker <url> is required")
	}
	username := extractArg(args, "u", "user")
	if username == "" {
		return SignalNone, errors.New("login: -u/--user <username> is required")
	}
	if !keys.HaveKeys() {
		return SignalNone, errors.New("login: no keys found — run register first")
	}
	pendingLoginBroker = brokerURL
	pendingLoginUsername = username
	return SignalNeedPassword, nil
}

// CompleteLogin finishes the login flow with the password the user entered.
func CompleteLogin(password string) (string, Signal, error) {
	broker := pendingLoginBroker
	username := pendingLoginUsername
	pendingLoginBroker = ""
	pendingLoginUsername = ""

	token, err := connection.Login(broker, username, password)
	if err != nil {
		return "", SignalNone, err
	}
	if err := connection.Connect(broker, username, token); err != nil {
		return "", SignalNone, fmt.Errorf("connect: %w", err)
	}
	return fmt.Sprintf("logged in to %s as %q", connection.BrokerURL(), username), SignalConnected, nil
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
