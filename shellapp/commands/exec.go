package commands

import (
	"errors"
	"fmt"
	"strings"
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
	case "wcid":
		return wcid(parsedCommand.Args), SignalNone, nil
	default:
		return "", SignalNone, errors.New("unknown command: " + parsedCommand.Name)
	}
}
