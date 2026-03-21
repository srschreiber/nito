package commands

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/keys"
	"github.com/srschreiber/nito/utils"
	wstypes "github.com/srschreiber/nito/websocket_types"
)

func wcid(args []Argument) string {
	// check for -c/--command filter
	filter := ""
	for _, a := range args {
		if a.Name == "c" || a.Name == "command" {
			if len(a.Values) > 0 {
				filter = strings.ToLower(a.Values[0])
			}
		}
	}

	var lines []string
	for _, cmd := range Registry {
		if filter != "" && cmd.Name != filter {
			continue
		}
		// first, replace all newlines in description with newline + tab
		cmd.Desc = strings.ReplaceAll(cmd.Desc, "\n", "\n\t")
		line := fmt.Sprintf("%s\n\t%s", cmd.Name, cmd.Desc)
		lines = append(lines, line)
		for _, arg := range cmd.Args {
			var flag string
			switch {
			case arg.Short != "" && arg.Long != "":
				flag = fmt.Sprintf("-%s / --%s", arg.Short, arg.Long)
			case arg.Long != "":
				flag = "--" + arg.Long
			default:
				flag = "-" + arg.Short
			}
			lines = append(lines, fmt.Sprintf("\t%s  %s", flag, arg.Desc))
		}
		// add newline separator between commands
		lines = append(lines, "")
	}
	if len(lines) == 0 {
		return "unknown command: " + filter
	}
	return strings.Join(lines, "\n")
}

var parser = NewParser()

func extractArg(args []Argument, short, long string) string {
	for _, a := range args {
		if a.Name == short || a.Name == long {
			if len(a.Values) > 0 {
				return a.Values[0]
			}
		}
	}
	return ""
}

// extractArgValues returns all values for a flag, supporting multi-word inputs
// like `echo -m hello world` where Values = ["hello", "world"].
func extractArgValues(args []Argument, short, long string) []string {
	for _, a := range args {
		if a.Name == short || a.Name == long {
			return a.Values
		}
	}
	return nil
}

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
	sig, err := keys.Sign(s.UserID + ":echo")
	if err != nil {
		return "", fmt.Errorf("echo: sign: %w", err)
	}
	msg := wstypes.ToBrokerWsMessage{
		RPCName:   "echo",
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

func roomCreateCmd(args []Argument) (string, error) {
	name := strings.Join(extractArgValues(args, "n", "name"), " ")
	if name == "" {
		return "", errors.New("room-create: -n/--name <name> is required")
	}

	roomKey, err := keys.GenerateRoomKey()
	if err != nil {
		return "", fmt.Errorf("room-create: %w", err)
	}

	encryptedKey, err := keys.EncryptRoomKey(roomKey)
	if err != nil {
		return "", fmt.Errorf("room-create: %w", err)
	}

	id, roomName, err := connection.CreateRoom(name, encryptedKey)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("room %q created (id: %s)", roomName, id), nil
}

func roomListCmd() (string, error) {
	rooms, err := connection.ListRooms()
	if err != nil {
		return "", err
	}
	if len(rooms) == 0 {
		return "no rooms", nil
	}
	var lines []string
	for _, r := range rooms {
		owner := ""
		if r.IsOwner {
			owner = " (owner)"
		}
		lines = append(lines, fmt.Sprintf("%s  %s%s", r.ID, r.Name, owner))
	}
	return strings.Join(lines, "\n"), nil
}

func roomSelectCmd(args []Argument) (string, Signal, error) {
	query := extractArg(args, "r", "room")
	if query == "" {
		return "", SignalNone, errors.New("room-select: -r/--room <name or id> is required")
	}
	rooms, err := connection.ListRooms()
	if err != nil {
		return "", SignalNone, err
	}
	var matched []string
	for _, r := range rooms {
		if r.ID == query || strings.HasPrefix(r.ID, query) || strings.EqualFold(r.Name, query) {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 {
		return "", SignalNone, fmt.Errorf("room-select: no room matching %q", query)
	}
	if len(matched) > 1 {
		return "", SignalNone, fmt.Errorf("room-select: ambiguous: %d rooms match %q", len(matched), query)
	}
	connection.SetCurrentRoom(matched[0])
	return fmt.Sprintf("selected room %s", matched[0]), SignalRoomSelected, nil
}

func roomInviteCmd(args []Argument) (string, error) {
	username := extractArg(args, "u", "user")
	if username == "" {
		return "", errors.New("room-invite: -u/--user <username> is required")
	}
	roomID := utils.DerefOrZero(connection.GetCurrentRoomID())
	if roomID == "" {
		return "", errors.New("room-invite: no room selected (use room-select or select in UI)")
	}

	// Fetch our own encrypted room key, decrypt it, then re-encrypt for the invitee.
	encryptedKey, err := connection.GetMyRoomKey(roomID)
	if err != nil {
		return "", fmt.Errorf("room-invite: fetch room key: %w", err)
	}
	roomKey, err := keys.DecryptRoomKey(encryptedKey)
	if err != nil {
		return "", fmt.Errorf("room-invite: decrypt room key: %w", err)
	}
	inviteePub, err := connection.GetUserPublicKey(username)
	if err != nil {
		return "", fmt.Errorf("room-invite: get invitee public key: %w", err)
	}
	encryptedForInvitee, err := keys.EncryptRoomKeyForPEM(roomKey, inviteePub)
	if err != nil {
		return "", fmt.Errorf("room-invite: encrypt for invitee: %w", err)
	}
	if err := connection.InviteUser(roomID, username, encryptedForInvitee); err != nil {
		return "", err
	}
	return fmt.Sprintf("invited %s to room %s", username, roomID), nil
}

func roomInvitesCmd() (string, error) {
	invites, err := connection.ListPendingInvites()
	if err != nil {
		return "", err
	}
	if len(invites) == 0 {
		return "no pending invites", nil
	}
	var lines []string
	for _, inv := range invites {
		lines = append(lines, fmt.Sprintf("%s  %s", inv.RoomID, inv.RoomName))
	}
	return strings.Join(lines, "\n"), nil
}

func roomAcceptCmd(args []Argument) (string, error) {
	roomID := extractArg(args, "r", "room")
	if roomID == "" {
		return "", errors.New("room-accept: -r/--room <room_id> is required")
	}
	if err := connection.AcceptInvite(roomID); err != nil {
		return "", err
	}
	return fmt.Sprintf("joined room %s", roomID), nil
}

func sayCmd(args []Argument) (string, error) {
	s := connection.CurrentSession()
	if s == nil {
		return "", errors.New("say: not connected (use connect first)")
	}
	roomID := utils.DerefOrZero(connection.GetCurrentRoomID())
	if roomID == "" {
		return "", errors.New("say: no room selected (use room-select first)")
	}
	text := strings.Join(extractArgValues(args, "m", "message"), " ")
	if text == "" {
		return "", errors.New("say: -m/--message <text> is required")
	}

	encRoomKey, err := connection.GetMyRoomKey(roomID)
	if err != nil {
		return "", fmt.Errorf("say: get room key: %w", err)
	}
	roomKey, err := keys.DecryptRoomKey(encRoomKey)
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
		FromUserID:    s.UserID,
		EncryptedText: base64.StdEncoding.EncodeToString(ciphertext),
	})
	if err != nil {
		return "", fmt.Errorf("say: %w", err)
	}
	sig, err := keys.Sign(s.UserID + ":room_message")
	if err != nil {
		return "", fmt.Errorf("say: sign: %w", err)
	}
	msg := wstypes.ToBrokerWsMessage{
		RPCName:   "room_message",
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

// ExecCommand takes a raw command string, parses it, and executes the corresponding action.
// Returns:
// - output: the string output to display to the user (if any)
// - signal: an optional status code (e.g., for success/failure)
// - error: any error that occurred during command execution
func ExecCommand(cmd string) (string, Signal, error) {
	parsedCommand, err := parser.ParseCommand(cmd)
	if err != nil {
		return "", 0, err
	}

	switch strings.ToLower(parsedCommand.Name) {
	case "clear":
		return "", SignalClear, nil
	case "exit":
		return "Exiting the shell...", SignalExit, nil
	case "history":
		return "Command history is not implemented yet.", SignalNone, nil
	case "echo":
		out, err := echoCmd(parsedCommand.Args)
		return out, SignalNone, err
	case "register":
		out, err := registerCmd(parsedCommand.Args)
		return out, SignalNone, err
	case "connect":
		out, err := connectCmd(parsedCommand.Args)
		if err != nil {
			return out, SignalNone, err
		}
		return out, SignalConnected, nil
	case "ping":
		out, err := ping(parsedCommand.Args)
		return out, SignalNone, err
	case "wcid":
		return wcid(parsedCommand.Args), SignalNone, nil
	case "room-create":
		out, err := roomCreateCmd(parsedCommand.Args)
		if err != nil {
			return "", SignalNone, err
		}
		return out, SignalRefreshRooms, nil
	case "room-list":
		out, err := roomListCmd()
		return out, SignalNone, err
	case "room-select":
		return roomSelectCmd(parsedCommand.Args)
	case "room-invite":
		out, err := roomInviteCmd(parsedCommand.Args)
		return out, SignalNone, err
	case "room-invites":
		out, err := roomInvitesCmd()
		return out, SignalNone, err
	case "room-accept":
		out, err := roomAcceptCmd(parsedCommand.Args)
		if err != nil {
			return "", SignalNone, err
		}
		return out, SignalRefreshRooms, nil
	case "say":
		out, err := sayCmd(parsedCommand.Args)
		return out, SignalNone, err
	default:
		return "", SignalNone, errors.New("unknown command: " + parsedCommand.Name)
	}
}
