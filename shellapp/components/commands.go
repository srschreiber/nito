package components

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/srschreiber/nito/shellapp/commands"
	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/history"
	"github.com/srschreiber/nito/shellapp/styles"
	"github.com/srschreiber/nito/shellapp/types"
)

const maxCmdHistory = 20

type cursorBlinkMsg struct{ gen int }

const (
	placeholderCmd  = "Type a command... (try: wcid)"
	placeholderChat = "Chat (/cmd to return to command mode)"
)

type CommandComponent struct {
	Placeholder    string
	focused        bool
	chatMode       bool
	textFieldValue string
	cursorPos      int
	cursorVisible  bool
	blinkGen       int
	width          int
	// command history (up/down navigation)
	cmdHistory []string
	historyIdx int    // -1 = not navigating
	draftText  string // saved input before navigating history
}

func NewCommandComponent(width int) *CommandComponent {
	return &CommandComponent{
		Placeholder:   placeholderCmd,
		cursorVisible: true,
		historyIdx:    -1,
		width:         width,
		cmdHistory:    history.Load(),
	}
}

func (c *CommandComponent) SetWidth(width int) {
	c.width = width
}

func (l *CommandComponent) newBlinkCmd() tea.Cmd {
	gen := l.blinkGen
	return tea.Tick(time.Millisecond*530, func(time.Time) tea.Msg {
		return cursorBlinkMsg{gen: gen}
	})
}

// resetCursor makes the cursor immediately visible and restarts the blink cycle.
func (l *CommandComponent) resetCursor() tea.Cmd {
	l.cursorVisible = true
	l.blinkGen++
	return l.newBlinkCmd()
}

func (l *CommandComponent) Init() tea.Cmd {
	return l.newBlinkCmd()
}

func (l *CommandComponent) SetFocused(focused bool) {
	l.focused = focused
}

func (l *CommandComponent) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case types.RoomSelectedMsg:
		return func() tea.Msg {
			return AppendHistoryMsg{Entries: []historyEntry{
				{text: "switched to room " + msg.RoomID + " — use /chat to switch to chat mode", isResponse: true},
			}}
		}
	case cursorBlinkMsg:
		if msg.gen != l.blinkGen {
			return nil // stale tick from before last reset
		}
		l.cursorVisible = !l.cursorVisible
		return l.newBlinkCmd()

	case tea.PasteMsg:
		text := msg.Content
		if text != "" {
			runes := []rune(l.textFieldValue)
			l.textFieldValue = string(runes[:l.cursorPos]) + text + string(runes[l.cursorPos:])
			l.cursorPos += len([]rune(text))
			return l.resetCursor()
		}

	case tea.KeyPressMsg:
		// Every key interaction resets the cursor to visible.
		blink := l.resetCursor()

		var keyCmd tea.Cmd
		switch msg.String() {
		case "left", "ctrl+b":
			if l.cursorPos > 0 {
				l.cursorPos--
			}
		case "right", "ctrl+f":
			if l.cursorPos < len([]rune(l.textFieldValue)) {
				l.cursorPos++
			}
		case "ctrl+a":
			l.cursorPos = 0
		case "ctrl+e":
			l.cursorPos = len([]rune(l.textFieldValue))
		case "ctrl+k":
			l.textFieldValue = string([]rune(l.textFieldValue)[:l.cursorPos])
		case "ctrl+d":
			runes := []rune(l.textFieldValue)
			if l.cursorPos < len(runes) {
				l.textFieldValue = string(append(runes[:l.cursorPos], runes[l.cursorPos+1:]...))
			}
		case "up":
			if len(l.cmdHistory) > 0 {
				if l.historyIdx == -1 {
					l.draftText = l.textFieldValue
					l.historyIdx = len(l.cmdHistory) - 1
				} else if l.historyIdx > 0 {
					l.historyIdx--
				}
				l.textFieldValue = l.cmdHistory[l.historyIdx]
				l.cursorPos = len([]rune(l.textFieldValue))
			}
		case "down":
			if l.historyIdx != -1 {
				if l.historyIdx == len(l.cmdHistory)-1 {
					l.historyIdx = -1
					l.textFieldValue = l.draftText
				} else {
					l.historyIdx++
					l.textFieldValue = l.cmdHistory[l.historyIdx]
				}
				l.cursorPos = len([]rune(l.textFieldValue))
			}
		case "enter":
			if l.textFieldValue != "" {
				keyCmd = l.handleEnter()
			}
		case "backspace":
			if l.cursorPos > 0 {
				runes := []rune(l.textFieldValue)
				runes = append(runes[:l.cursorPos-1], runes[l.cursorPos:]...)
				l.textFieldValue = string(runes)
				l.cursorPos--
			}
		default:
			text := msg.Key().Text
			if text != "" {
				runes := []rune(l.textFieldValue)
				l.textFieldValue = string(runes[:l.cursorPos]) + text + string(runes[l.cursorPos:])
				l.cursorPos += len([]rune(text))
			}
		}

		if keyCmd != nil {
			return tea.Batch(keyCmd, blink)
		}
		return blink
	}
	return nil
}

func (l *CommandComponent) handleEnter() tea.Cmd {
	input := l.textFieldValue

	l.cmdHistory = append(l.cmdHistory, input)
	if len(l.cmdHistory) > maxCmdHistory {
		l.cmdHistory = l.cmdHistory[1:]
	}
	l.historyIdx = -1
	l.textFieldValue = ""
	l.cursorPos = 0

	// Mode-switch commands are intercepted before anything else.
	if input == "/chat" {
		l.chatMode = true
		l.Placeholder = placeholderChat
		return func() tea.Msg {
			return AppendHistoryMsg{Entries: []historyEntry{
				{text: "> " + input},
				{text: "Switched to chat mode. Type messages and press enter to send.", isResponse: true},
			}}
		}
	}
	if input == "/cmd" {
		l.chatMode = false
		l.Placeholder = placeholderCmd
		return func() tea.Msg {
			return AppendHistoryMsg{Entries: []historyEntry{
				{text: "> " + input},
				{text: "Switched to command mode.", isResponse: true},
			}}
		}
	}

	// In chat mode, plain input is sent as a room message.
	if l.chatMode {
		entries := []historyEntry{{text: "[you]: " + input}}
		if err := commands.SendRoomMessage(input); err != nil {
			entries = append(entries, historyEntry{text: err.Error(), isResponse: true})
		}
		return func() tea.Msg { return AppendHistoryMsg{Entries: entries} }
	}

	entries := []historyEntry{
		{text: "> " + input},
	}

	output, signal, err := commands.ExecCommand(input)
	if err != nil {
		entries = append(entries, historyEntry{text: err.Error(), isResponse: true})
	} else if output != "" {
		entries = append(entries, historyEntry{text: output, isResponse: true})
	}

	userID := ""
	if s := connection.CurrentSession(); s != nil {
		userID = s.UserID
	}
	connMsg := types.ConnectionStatusMsg{
		Connected: connection.IsConnected(),
		BrokerURL: connection.BrokerURL(),
		UserID:    userID,
	}
	emitConn := func() tea.Msg { return connMsg }

	switch signal {
	case commands.SignalClear:
		return tea.Batch(func() tea.Msg { return ClearHistoryMsg{} }, emitConn)
	case commands.SignalExit:
		_ = history.Save(l.cmdHistory)
		return tea.Quit
	case commands.SignalRefreshRooms:
		return tea.Batch(
			func() tea.Msg { return AppendHistoryMsg{Entries: entries} },
			emitConn,
			func() tea.Msg { return types.RoomsFetchMsg{} },
		)
	case commands.SignalConnected:
		return tea.Batch(
			func() tea.Msg { return AppendHistoryMsg{Entries: entries} },
			emitConn,
			func() tea.Msg { return types.ConnectedMsg{} },
		)
	case commands.SignalRoomSelected:
		roomID := connection.GetSessionRoomID()
		if roomID == nil {
			break
		}
		id := *roomID
		entries = append(entries, historyEntry{text: "use /chat to switch to chat mode", isResponse: true})
		return tea.Batch(
			func() tea.Msg { return AppendHistoryMsg{Entries: entries} },
			emitConn,
			func() tea.Msg { return types.RoomSelectedMsg{RoomID: id} },
		)
	}

	return tea.Batch(func() tea.Msg { return AppendHistoryMsg{Entries: entries} }, emitConn)
}

func (l *CommandComponent) Render() string {
	prompt := styles.PromptStyle.Render("> ")
	runes := []rune(l.textFieldValue)

	var render string
	if l.focused && l.cursorVisible {
		before := string(runes[:l.cursorPos])
		if l.cursorPos < len(runes) {
			underCursor := styles.CursorHighlightStyle.Render(string(runes[l.cursorPos]))
			render = prompt + before + underCursor + string(runes[l.cursorPos+1:])
		} else if len(runes) > 0 {
			render = prompt + before + styles.CursorHighlightStyle.Render(" ")
		} else {
			// Empty input — highlight the first char of the placeholder in place
			pr := []rune(l.Placeholder)
			render = prompt + styles.CursorHighlightStyle.Render(string(pr[0])) + styles.Grey.Render(string(pr[1:]))
		}
	} else {
		if len(runes) > 0 {
			render = prompt + string(runes)
		} else {
			render = prompt + styles.Grey.Render(l.Placeholder)
		}
	}

	style := styles.UnfocusedBorderStyle
	if l.focused {
		style = styles.FocusedBorderStyle
	}
	if l.width > 0 {
		style = style.Width(l.width)
	}
	return style.Render(render)
}
