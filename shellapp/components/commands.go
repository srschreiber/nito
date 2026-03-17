package components

import (
	"strings"
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

type CommandComponent struct {
	Placeholder    string
	focused        bool
	textFieldValue string
	cursorPos      int
	cursorVisible  bool
	blinkGen       int
	width          int
	histWrapWidth  int // usable text width inside the history box
	// command history (up/down navigation)
	cmdHistory []string
	historyIdx int    // -1 = not navigating
	draftText  string // saved input before navigating history
}

func NewCommandComponent(width int) *CommandComponent {
	return &CommandComponent{
		Placeholder:   "Type a command... (try: wcid)",
		cursorVisible: true,
		historyIdx:    -1,
		width:         width,
		histWrapWidth: histWrapFromWidth(width),
		cmdHistory:    history.Load(),
	}
}

// histWrapFromWidth derives the text wrap width from the history content width.
// The history box uses Padding(0,1) so each side subtracts 1 from usable width.
func histWrapFromWidth(histWidth int) int {
	w := histWidth - 2
	if w < 1 {
		return 1
	}
	return w
}

func (c *CommandComponent) SetWidth(cmdWidth, histWidth int) {
	c.width = cmdWidth
	c.histWrapWidth = histWrapFromWidth(histWidth)
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

// wrapText splits s into chunks of at most width runes.
func wrapText(s string, width int) []string {
	runes := []rune(s)
	if len(runes) <= width {
		return []string{s}
	}
	var lines []string
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		lines = append(lines, string(runes))
	}
	return lines
}

func (l *CommandComponent) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case cursorBlinkMsg:
		if msg.gen != l.blinkGen {
			return nil // stale tick from before last reset
		}
		l.cursorVisible = !l.cursorVisible
		return l.newBlinkCmd()

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
	cmd := l.textFieldValue

	l.cmdHistory = append(l.cmdHistory, cmd)
	if len(l.cmdHistory) > maxCmdHistory {
		l.cmdHistory = l.cmdHistory[1:]
	}
	l.historyIdx = -1
	l.textFieldValue = ""
	l.cursorPos = 0

	var entries []historyEntry
	for i, line := range wrapText(cmd, l.histWrapWidth) {
		if i == 0 {
			entries = append(entries, historyEntry{text: "> " + line})
		} else {
			entries = append(entries, historyEntry{text: "  " + line})
		}
	}

	output, signal, err := commands.ExecCommand(cmd)
	if err != nil {
		for _, para := range strings.Split(err.Error(), "\n") {
			for _, line := range wrapText(para, l.histWrapWidth) {
				entries = append(entries, historyEntry{text: line, isResponse: true})
			}
		}
	} else if output != "" {
		for _, para := range strings.Split(output, "\n") {
			for _, line := range wrapText(para, l.histWrapWidth) {
				entries = append(entries, historyEntry{text: line, isResponse: true})
			}
		}
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
