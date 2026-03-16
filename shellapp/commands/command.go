package commands

import (
	"errors"
	"strings"
)

type Signal int

const (
	SignalNone  Signal = 0
	SignalExit  Signal = 1
	SignalClear Signal = 2
)

type ArgDef struct {
	Short string // e.g. "f"
	Long  string // e.g. "force"
	Desc  string
}

type CommandDef struct {
	Name string
	Desc string
	Args []ArgDef
}

var Registry = []CommandDef{
	{Name: "clear", Desc: "clear the screen"},
	{Name: "exit", Desc: "exit the shell"},
	{Name: "history", Desc: "show command history"},
	{Name: "connect", Desc: "establish a persistent WebSocket connection to a broker", Args: []ArgDef{
		{Short: "b", Long: "broker", Desc: "broker base URL (e.g. localhost:8080)"},
	}},
	{Name: "ping", Desc: "test connectivity to a broker via WebSocket", Args: []ArgDef{
		{Short: "b", Long: "broker", Desc: "broker base URL (e.g. localhost:7070)"},
	}},
	{Name: "wcid", Desc: "describe all commands and their arguments", Args: []ArgDef{
		{Short: "c", Long: "command", Desc: "show details for a specific command"},
	}},
}

var CommandNames = func() map[string]interface{} {
	m := make(map[string]interface{}, len(Registry))
	for _, c := range Registry {
		m[c.Name] = nil
	}
	return m
}()

type ArgumentType string

const (
	ArgumentLongForm  = ArgumentType("long")
	ArgumentShortForm = ArgumentType("short")
)

type Argument struct {
	Name   string
	Values []string
	Type   ArgumentType
}

type Command struct {
	Name string
	Args []Argument
}

type CommandParser struct {
	lastTokSeen int
}

func NewParser() *CommandParser {
	return &CommandParser{}
}

// ParseCommand takes a raw input string and parses it into a Command struct.
// A command follows a simple structure:
// [COMMAND_NAME] [ARG1] [ARG2] ... [ARGN]
func (pc *CommandParser) ParseCommand(input string) (Command, error) {
	input = strings.TrimSpace(input)
	i := 0

	commandName := ""
	commandArgs := []*Argument{}

	var previousArg *Argument = nil

	for i < len(input) {
		c := input[i]

		switch c {
		case ' ', '\t', '\n':
			i += 1
			continue
		case '-':
			pc.lastTokSeen = i
			argumentType := ArgumentShortForm
			// check if next is also a '-'
			if i+1 < len(input) && input[i+1] == '-' {
				argumentType = ArgumentLongForm
				pc.lastTokSeen = i + 1
			}

			// possibly get a second - for long form
			argName, err := pc.parseString(input)
			if err != nil {
				return Command{}, err
			}

			arg := Argument{
				Name: argName,
				Type: argumentType,
			}

			// append it to args, set the running arg in case scan in more params for it
			previousArg = &arg
			commandArgs = append(commandArgs, &arg)
			i = pc.lastTokSeen
		default:
			if previousArg != nil {
				pc.lastTokSeen = i - 1
				value, err := pc.parseString(input)
				if err != nil {
					return Command{}, err
				}
				previousArg.Values = append(previousArg.Values, value)
				i = pc.lastTokSeen
			} else if commandName == "" {
				pc.lastTokSeen = i - 1
				name, err := pc.parseString(input)
				if err != nil {
					return Command{}, err
				}
				commandName = strings.ToLower(name)
				if _, ok := CommandNames[commandName]; !ok {
					return Command{}, errors.New("unknown command: " + commandName)
				}
				i = pc.lastTokSeen
			} else {
				return Command{}, errors.New("unexpected token: " + string(c))
			}
		}
	}

	// build the final command
	ret := Command{}
	ret.Name = commandName
	ret.Args = make([]Argument, len(commandArgs))
	for c, arg := range commandArgs {
		ret.Args[c].Name = arg.Name
		ret.Args[c].Type = arg.Type
		ret.Args[c].Values = arg.Values
	}

	return ret, nil

}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n'
}

func (pc *CommandParser) parseString(input string) (string, error) {
	// scan until first whitespace after lastTokSeen
	i := pc.lastTokSeen + 1
	for i < len(input) && !isWhitespace(input[i]) {
		i++
	}

	str := input[pc.lastTokSeen+1 : i]

	pc.lastTokSeen = i
	return str, nil
}
