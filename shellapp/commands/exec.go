package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	brokertypes "github.com/srschreiber/nito/broker/types"
	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/keys"
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

	payload, err := json.Marshal(brokertypes.EchoPayload{Text: text})
	if err != nil {
		return "", fmt.Errorf("echo: %w", err)
	}
	msg := brokertypes.WebsocketMessage{
		RPCName:   "echo",
		RequestID: fmt.Sprintf("%d", time.Now().UnixNano()),
		UserID:    s.UserID,
		Nonce:     fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("echo: %w", err)
	}

	if err := connection.Send(data); err != nil {
		return "", fmt.Errorf("echo: send: %w", err)
	}

	resp, err := connection.Receive(5 * time.Second)
	if err != nil {
		return "", fmt.Errorf("echo: receive: %w", err)
	}

	var respMsg brokertypes.WebsocketMessage
	if err := json.Unmarshal(resp, &respMsg); err != nil {
		return "", fmt.Errorf("echo: bad response: %w", err)
	}

	var respPayload brokertypes.EchoPayload
	if err := json.Unmarshal(respMsg.Payload, &respPayload); err != nil {
		return "", fmt.Errorf("echo: bad payload: %w", err)
	}

	return respPayload.Text, nil
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
		return out, SignalNone, err
	case "ping":
		out, err := ping(parsedCommand.Args)
		return out, SignalNone, err
	case "wcid":
		return wcid(parsedCommand.Args), SignalNone, nil
	default:
		return "", SignalNone, errors.New("unknown command: " + parsedCommand.Name)
	}
}
