package components

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/styles"
	"github.com/srschreiber/nito/shellapp/types"
	"github.com/srschreiber/nito/utils"
)

type roomsPollMsg struct{}

type RoomsComponent struct {
	rooms    []types.RoomEntry
	selected *string
	cursor   int
	focused  bool
	width    int
	height   int
}

func NewRoomsComponent(width, height int) *RoomsComponent {
	return &RoomsComponent{width: width, height: height}
}

func (r *RoomsComponent) SetSize(width, height int) {
	r.width = width
	r.height = height
}

func (r *RoomsComponent) SetFocused(focused bool) {
	r.focused = focused
}

func (r *RoomsComponent) Init() tea.Cmd {
	return tea.Batch(r.fetch(), r.schedulePoll())
}

func (r *RoomsComponent) schedulePoll() tea.Cmd {
	return tea.Tick(30*time.Second, func(time.Time) tea.Msg {
		return roomsPollMsg{}
	})
}

func (r *RoomsComponent) fetch() tea.Cmd {
	return func() tea.Msg {
		rooms, err := connection.ListRooms()
		if err != nil {
			return nil
		}
		return types.RoomsUpdatedMsg{Rooms: rooms}
	}
}

func (r *RoomsComponent) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case roomsPollMsg:
		return tea.Batch(r.fetch(), r.schedulePoll())
	case types.RoomsFetchMsg:
		return r.fetch()
	case types.ConnectionStatusMsg:
		if msg.Connected {
			return r.fetch()
		}
	case types.RoomsUpdatedMsg:
		r.rooms = msg.Rooms
		if r.cursor >= len(r.rooms) {
			r.cursor = 0
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if r.cursor > 0 {
				r.cursor--
			}
		case "down", "j":
			if r.cursor < len(r.rooms)-1 {
				r.cursor++
			}
		case "enter":
			if len(r.rooms) > 0 {
				room := r.rooms[r.cursor]
				selected := room.ID
				r.selected = &selected
			}
		}
	}
	return nil
}

func (r *RoomsComponent) Render() string {
	title := styles.SectionTitleStyle.Render("Rooms")
	body := title + "\n"

	if len(r.rooms) == 0 {
		body += styles.Grey.Render("  no rooms")
	} else {
		for i, room := range r.rooms {
			name := room.Name
			if room.IsOwner {
				name += " " + styles.Grey.Render("(owner)")
			}
			cursor := "  "
			if room.ID == utils.DerefOrZero(r.selected) {
				checked := styles.SelectedStyle.Render("✓")
				name = fmt.Sprintf("%s (%s)", name, checked)
			}
			if i == r.cursor {
				cursor = styles.CursorStyle.Render("› ")
			}
			body += styles.ItemStyle.Render(cursor+name) + "\n"
		}
	}

	borderColor := lipgloss.Color("238")
	if r.focused {
		borderColor = lipgloss.Color("213")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(r.width).
		Height(r.height).
		Render(body)
}
