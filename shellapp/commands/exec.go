package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/srschreiber/nito/shellapp/connection"
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
		line := fmt.Sprintf("/%s  %s", cmd.Name, cmd.Desc)
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
			lines = append(lines, fmt.Sprintf("  %s  %s", flag, arg.Desc))
		}
	}
	if len(lines) == 0 {
		return "unknown command: " + filter
	}
	return strings.Join(lines, "\n")
}

var parser = NewParser()

func connectCmd(args []Argument) (string, error) {
	brokerURL := ""
	for _, a := range args {
		if a.Name == "b" || a.Name == "broker" {
			if len(a.Values) > 0 {
				brokerURL = a.Values[0]
			}
		}
	}
	if brokerURL == "" {
		return "", errors.New("connect: -b/--broker <url> is required")
	}

	if err := connection.Connect(brokerURL); err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}

	return "connected to " + connection.BrokerURL(), nil
}

func ping(args []Argument) (string, error) {
	brokerURL := ""
	for _, a := range args {
		if a.Name == "b" || a.Name == "broker" {
			if len(a.Values) > 0 {
				brokerURL = a.Values[0]
			}
		}
	}
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
// Right now, everything is hardcoded
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
