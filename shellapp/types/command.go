package types

import (
	"errors"
	"strings"
)

var CommandNames = map[string]interface{}{
	"help":    nil,
	"clear":   nil,
	"exit":    nil,
	"history": nil,
}

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

// ParseCommand takes a raw input string and parses it into a Command struct.
// A command follows a simple structure:
// /[COMMAND_NAME] [ARG1] [ARG2] ... [ARGN]
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
		case '/':
			pc.lastTokSeen = i
			name, err := pc.parseString(input)
			if err != nil {
				return Command{}, err
			}
			commandName = strings.ToLower(name)
			// validate command name
			if _, ok := CommandNames[commandName]; !ok {
				return Command{}, errors.New("unknown command: " + commandName)
			}
			i = pc.lastTokSeen
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
			// if we have a previous arg, this is a value for it
			if previousArg != nil {
				pc.lastTokSeen = i - 1
				value, err := pc.parseString(input)
				if err != nil {
					return Command{}, err
				}
				previousArg.Values = append(previousArg.Values, value)
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
