package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/srschreiber/nito/shellapp/styles"
)

type historyEntry struct {
	text       string // raw text, may contain newlines/tabs; wrapped at render time
	isResponse bool
}

// AppendHistoryMsg is emitted by CommandComponent when entries should be added.
type AppendHistoryMsg struct {
	Entries []historyEntry
}

// ClearHistoryMsg is emitted by CommandComponent when history should be cleared.
type ClearHistoryMsg struct{}

// NewResponseAppendMsg builds an AppendHistoryMsg for a single server-response entry.
func NewResponseAppendMsg(text string) AppendHistoryMsg {
	return AppendHistoryMsg{Entries: []historyEntry{{text: text, isResponse: true}}}
}

type ConversationHistory struct {
	entries []historyEntry
	scroll  int // lines scrolled up from the bottom (0 = pinned to bottom)
	focused bool
	width   int // content width passed from layout (lipgloss Width value)
	height  int // content height passed from layout (lipgloss Height value)
}

func NewConversationHistory(width, height int) *ConversationHistory {
	return &ConversationHistory{width: width, height: height}
}

func (h *ConversationHistory) SetSize(width, height int) {
	h.width = width
	h.height = height
}

func (h *ConversationHistory) Init() tea.Cmd { return nil }

func (h *ConversationHistory) SetFocused(focused bool) {
	h.focused = focused
}

// textWidth is the usable text column width inside the border and padding.
// The style uses Padding(0,1) so each horizontal side consumes 1 column.
func (h *ConversationHistory) textWidth() int {
	w := h.width - 2
	if w < 1 {
		return 1
	}
	return w
}

// wrapEntry splits one raw entry into display-ready styled lines, hard-wrapping at tw.
func wrapEntry(e historyEntry, tw int) []string {
	raw := strings.ReplaceAll(e.text, "\t", "    ")
	var lines []string
	for _, para := range strings.Split(raw, "\n") {
		runes := []rune(para)
		if len(runes) == 0 {
			lines = append(lines, "")
			continue
		}
		for len(runes) > tw {
			lines = append(lines, string(runes[:tw]))
			runes = runes[tw:]
		}
		lines = append(lines, string(runes))
	}
	return lines
}

// allLines expands all entries into a flat, styled slice of display strings.
// Always derived from raw entries so terminal resize is handled automatically.
func (h *ConversationHistory) allLines() []string {
	tw := h.textWidth()
	var lines []string
	for _, e := range h.entries {
		for _, l := range wrapEntry(e, tw) {
			if e.isResponse {
				lines = append(lines, styles.ResponseStyle.Render(l))
			} else {
				lines = append(lines, styles.Grey.Render(l))
			}
		}
	}
	return lines
}

// contentBudget is the number of rows available for scrollable content.
// Fixed rows consumed: 1 (title) + 1 (status line) = 2.
func (h *ConversationHistory) contentBudget() int {
	b := h.height - 2
	if b < 1 {
		return 1
	}
	// give a little buffer too. IDK why we need to do this, but it fixes bugs and it hurts my head
	// trying to figure out why we need it, so I'm just gonna leave it here ¯\_(ツ)_/¯
	b -= 10
	return b
}

// clampScroll ensures h.scroll stays within valid bounds given total line count.
func (h *ConversationHistory) clampScroll(total int) {
	maxScroll := total - h.contentBudget()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if h.scroll > maxScroll {
		h.scroll = maxScroll
	}
	if h.scroll < 0 {
		h.scroll = 0
	}
}

func (h *ConversationHistory) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case AppendHistoryMsg:
		h.entries = append(h.entries, msg.Entries...)
		h.scroll = 0 // pin to bottom on new content
	case ClearHistoryMsg:
		h.entries = nil
		h.scroll = 0
	case tea.KeyPressMsg:
		if !h.focused {
			return nil
		}
		lines := h.allLines()
		h.clampScroll(len(lines))
		budget := h.contentBudget()
		maxScroll := len(lines) - budget
		if maxScroll < 0 {
			maxScroll = 0
		}
		switch msg.String() {
		case "up":
			if h.scroll < maxScroll {
				h.scroll++
			}
		case "down":
			if h.scroll > 0 {
				h.scroll--
			}
		}
	}
	return nil
}

func (h *ConversationHistory) Render() string {
	// Fixed first row: title.
	rows := []string{styles.SectionTitleStyle.Render("Commands")}

	if len(h.entries) == 0 {
		rows = append(rows, styles.Grey.Render("No messages yet."))
	} else {
		lines := h.allLines()
		total := len(lines)
		h.clampScroll(total)

		budget := h.contentBudget()

		// Determine the bottom of the viewport.
		end := total - h.scroll
		if end > total {
			end = total
		}
		// Tentative start with full budget.
		start := end - budget
		if start < 0 {
			start = 0
		}

		showAbove := start > 0
		showBelow := end < total

		// Reserve rows for indicators.
		rows_ := budget
		if showAbove {
			rows_--
		}
		if showBelow {
			rows_--
		}
		if rows_ < 0 {
			rows_ = 0
		}

		// Recompute start with the tighter budget (end stays fixed).
		start = end - rows_
		if start < 0 {
			start = 0
		}

		end -= 8

		if showAbove {
			rows = append(rows, styles.Grey.Render("↑ more"))
		}
		rows = append(rows, lines[start:end]...)
		if showBelow {
			rows = append(rows, styles.Grey.Render("↓ more"))
		}

		// Fixed last row: position indicator.
		rows = append(rows, styles.LineStyle.Render(fmt.Sprintf("L%d/%d", end, total)))
	}

	borderColor := lipgloss.Color("238")
	if h.focused {
		borderColor = lipgloss.Color("213")
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(h.width).
		Height(h.height)
	return style.Render(strings.Join(rows, "\n"))
}
