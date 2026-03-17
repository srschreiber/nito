package components

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/srschreiber/nito/shellapp/styles"
)

type historyEntry struct {
	text       string
	isResponse bool
}

// AppendHistoryMsg is emitted by CommandComponent when entries should be added.
type AppendHistoryMsg struct {
	Entries []historyEntry
}

// ClearHistoryMsg is emitted by CommandComponent when history should be cleared.
type ClearHistoryMsg struct{}

type ConversationHistory struct {
	entries []historyEntry
	scroll  int
	focused bool
	width   int // lipgloss content width (excludes border+padding)
	height  int // lipgloss content height (excludes border)
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

// visibleRows is how many entry lines fit, leaving room for title + line indicator.
func (h *ConversationHistory) visibleRows() int {
	v := h.height - 2 // title row + line indicator row
	if v < 1 {
		return 1
	}
	return v
}

func (h *ConversationHistory) maxScroll() int {
	excess := len(h.entries) - h.visibleRows()
	if excess < 0 {
		return 0
	}
	return excess
}

func (h *ConversationHistory) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case AppendHistoryMsg:
		h.entries = append(h.entries, msg.Entries...)
		h.scroll = 0
	case ClearHistoryMsg:
		h.entries = nil
		h.scroll = 0
	case tea.KeyPressMsg:
		if !h.focused {
			return nil
		}
		switch msg.String() {
		case "up":
			h.scroll++
			if h.scroll > h.maxScroll() {
				h.scroll = h.maxScroll()
			}
		case "down":
			h.scroll--
			if h.scroll < 0 {
				h.scroll = 0
			}
		}
	}
	return nil
}

func (h *ConversationHistory) Render() string {
	render := styles.SectionTitleStyle.Render("Conversation") + "\n"

	if len(h.entries) == 0 {
		render += styles.Grey.Render("No messages yet.")
	} else {
		// rows available for entries + indicators (title and line counter always occupy 2)
		rows := h.height - 2

		// "↓ more" is known before we pick the window (depends only on scroll)
		showBelow := h.scroll > 0
		if showBelow {
			rows--
		}

		viewEnd := len(h.entries) - h.scroll
		viewStart := viewEnd - rows
		if viewStart < 0 {
			viewStart = 0
		}

		// "↑ more" depends on viewStart; if it will show, shrink the window by 1
		showAbove := viewStart > 0
		if showAbove {
			rows--
			viewStart = viewEnd - rows
			if viewStart < 0 {
				viewStart = 0
			}
		}

		if showAbove {
			render += styles.Grey.Render("↑ more") + "\n"
		}
		for i := viewStart; i < viewEnd; i++ {
			entry := h.entries[i]
			if entry.isResponse {
				render += styles.ResponseStyle.Render(entry.text) + "\n"
			} else {
				render += styles.Grey.Render(entry.text) + "\n"
			}
		}
		if showBelow {
			render += styles.Grey.Render("↓ more") + "\n"
		}
		render += styles.LineStyle.Render(fmt.Sprintf("L%d/%d", viewEnd, len(h.entries)))
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
	return style.Render(render)
}
