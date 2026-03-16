package components

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/srschreiber/nito/shellapp/styles"
	"github.com/srschreiber/nito/shellapp/types"
)

const (
	maxHistoryRows = 10
)

type cursorBlinkMsg struct{}

type CommandComponent struct {
	Placeholder    string
	History        []string
	focused        bool
	textFieldValue string
	cursorVisible  bool
	scroll         int // 0 = bottom (most recent), positive = scrolled up
}

func NewCommandComponent() *CommandComponent {
	return &CommandComponent{
		Placeholder:   "Type a command... (help for available commands)",
		cursorVisible: true,
	}
}

func cursorBlinkCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*530, func(t time.Time) tea.Msg {
		return cursorBlinkMsg{}
	})
}

func (l *CommandComponent) Init() tea.Cmd {
	return cursorBlinkCmd()
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

func (l *CommandComponent) maxScroll() int {
	excess := len(l.History) - maxHistoryRows
	if excess < 0 {
		return 0
	}
	return excess
}

func (l *CommandComponent) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case cursorBlinkMsg:
		l.cursorVisible = !l.cursorVisible
		return cursorBlinkCmd()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up":
			l.scroll++
			if l.scroll > l.maxScroll() {
				l.scroll = l.maxScroll()
			}
			return nil
		case "down":
			l.scroll--
			if l.scroll < 0 {
				l.scroll = 0
			}
			return nil
		case "enter":
			if l.textFieldValue != "" {
				for i, line := range wrapText(l.textFieldValue, types.ShellWrapWidth-10) {
					if i == 0 {
						l.History = append(l.History, "> "+line)
					} else {
						l.History = append(l.History, "  "+line)
					}
				}
				l.textFieldValue = ""
				l.scroll = 0
			}
		case "backspace":
			if len(l.textFieldValue) > 0 {
				l.textFieldValue = l.textFieldValue[:len(l.textFieldValue)-1]
			}
		default:
			l.textFieldValue += msg.Key().Text
		}
	}
	return nil
}

func (l *CommandComponent) Render() string {
	render := ""

	viewEnd := len(l.History) - l.scroll
	viewStart := viewEnd - maxHistoryRows
	if viewStart < 0 {
		viewStart = 0
	}

	canScrollUp := viewStart > 0
	canScrollDown := l.scroll > 0

	if canScrollUp {
		render += styles.Grey.Render("↑ more") + "\n"
	}

	for i := viewStart; i < viewEnd; i++ {
		render += styles.Grey.Render(l.History[i]) + "\n"
	}

	if canScrollDown {
		render += styles.Grey.Render("↓ more") + "\n"
	}

	// Blinking cursor — only when focused
	cursor := ""
	if l.focused && l.cursorVisible {
		cursor = styles.CursorStyle.Render("▋")
	} else if l.focused {
		cursor = " "
	}

	prompt := styles.PromptStyle.Render("> ")
	if l.textFieldValue != "" {
		wrapped := wrapText(l.textFieldValue, types.ShellWrapWidth-10)
		render += prompt + strings.Join(wrapped, "\n") + cursor
	} else {
		render += prompt + styles.Grey.Render(l.Placeholder) + cursor
	}

	if l.focused {
		return styles.FocusedBorderStyle.Render(render)
	}
	return styles.UnfocusedBorderStyle.Render(render)
}
