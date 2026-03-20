package main

import (
	"encoding/json"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/srschreiber/nito/shellapp/components"
	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/styles"
	"github.com/srschreiber/nito/shellapp/types"
	wstypes "github.com/srschreiber/nito/websocket_types"
)

// Box overhead constants (lipgloss borders + padding).
// History/Rooms/Status: RoundedBorder (all 4 sides) + Padding(0,1) → 4 wide, 2 tall.
// Command: ThickBorder (left only) + Padding(0,1) → 3 wide, 0 tall.
// AppStyle: Padding(1,2) → 4 wide, 2 tall.
const (
	histBoxOverheadW  = 4
	histBoxOverheadH  = 2
	rightBoxOverheadW = 4
	cmdBoxOverheadW   = 3
	appPaddingW       = 4
)

// layout holds computed content dimensions for each component.
type layout struct {
	histW, histH   int
	roomsW, roomsH int
	membersW       int // 0 when no room is selected
	statW, statH   int
	cmdW           int
}

// computeLayout derives component content dimensions from the terminal size.
// When showMembers is true the right column is split in half between rooms and members.
func computeLayout(termW, termH int, showMembers bool) layout {
	if termW < 30 {
		termW = 30
	}
	if termH < 12 {
		termH = 12
	}

	usableW := termW - appPaddingW
	pHistBoxW := .2
	pHistBoxH := .9
	histBoxW := int(float64(termW) * pHistBoxW)
	histBoxH := int(float64(termH) * pHistBoxH)

	histW := histBoxW - histBoxOverheadW
	histH := histBoxH - histBoxOverheadH

	rightBoxW := usableW - histBoxW

	// Split the right column: rooms 85%, status 15%.
	roomsBoxH := int(float64(histBoxH) * 0.85)
	statBoxH := histBoxH - roomsBoxH
	roomsH := roomsBoxH - histBoxOverheadH
	statH := statBoxH

	var roomsW, membersW int
	if showMembers {
		half := rightBoxW / 2
		roomsW = half - rightBoxOverheadW
		membersW = rightBoxW - half - rightBoxOverheadW
	} else {
		roomsW = rightBoxW - rightBoxOverheadW
		membersW = 0
	}
	statW := rightBoxW - rightBoxOverheadW

	cmdW := usableW - cmdBoxOverheadW

	if histW < 10 {
		histW = 10
	}
	if histH < 3 {
		histH = 3
	}
	if roomsW < 5 {
		roomsW = 5
	}
	if membersW < 0 {
		membersW = 0
	}
	if statW < 5 {
		statW = 5
	}
	if roomsH < 2 {
		roomsH = 2
	}
	if statH < 2 {
		statH = 2
	}
	if cmdW < 10 {
		cmdW = 10
	}

	return layout{
		histW: histW, histH: histH,
		roomsW: roomsW, roomsH: roomsH,
		membersW: membersW,
		statW:    statW, statH: statH,
		cmdW: cmdW,
	}
}

type model struct {
	history          *components.ConversationHistory
	rooms            *components.RoomsComponent
	members          *components.RoomMembersComponent
	status           *components.StatusComponent
	command          *components.CommandComponent
	comps            []components.Component
	focusable        []int
	focusedComponent int
	selectedRoomID   *string
	termW, termH     int
}

func initialModel() model {
	termW, termH := 120, 40
	l := computeLayout(termW, termH, false)
	history := components.NewConversationHistory(l.histW, l.histH)
	rooms := components.NewRoomsComponent(l.roomsW, l.roomsH)
	members := components.NewRoomMembersComponent(l.membersW, l.roomsH)
	status := components.NewStatusComponent(l.statW, l.statH)
	command := components.NewCommandComponent(l.cmdW)

	// comps: 0=history, 1=rooms, 2=members (display-only), 3=status (display-only), 4=command
	m := model{
		history:          history,
		rooms:            rooms,
		members:          members,
		status:           status,
		command:          command,
		comps:            []components.Component{history, rooms, members, status, command},
		focusable:        []int{0, 1, 4},
		focusedComponent: 2, // index into focusable → comps[4] = command
		termW:            termW,
		termH:            termH,
	}
	m.comps[m.focusable[m.focusedComponent]].SetFocused(true)
	return m
}

func (m *model) relayout(termW, termH int) {
	m.termW, m.termH = termW, termH
	l := computeLayout(termW, termH, m.selectedRoomID != nil)
	m.history.SetSize(l.histW, l.histH)
	m.rooms.SetSize(l.roomsW, l.roomsH)
	m.members.SetSize(l.membersW, l.roomsH)
	m.status.SetSize(l.statW, l.statH)
	m.command.SetWidth(l.cmdW, l.histW)
}

// notificationMsg is delivered to the model when the readLoop routes a
// server-push notification from the broker.
type notificationMsg wstypes.NotificationPayload

// // incomingWsMsg carries raw bytes of a non-notification WS message.
// type incomingWsMsg []byte
type echoWsMsg wstypes.EchoPayload

// waitNotification blocks on the notification channel the readLoop feeds and
// returns the text as a notificationMsg. The model re-arms this after each hit.
func waitNotification() tea.Cmd {
	return func() tea.Msg {
		ch := connection.NotifChan()
		if ch == nil {
			return nil
		}
		text, ok := <-ch
		if !ok {
			return nil
		}

		// conv to notificationMsg for type safety and to avoid string conversions in the readLoop.'
		var payload wstypes.NotificationPayload
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			fmt.Printf("waitNotification: unmarshal payload: %v\n", err)
			// to rearm
			return notificationMsg{}
		}
		return notificationMsg(payload)
	}
}

func waitEcho() tea.Cmd {
	return func() tea.Msg {
		ch := connection.EchoChan()
		if ch == nil {
			return nil
		}
		data, ok := <-ch
		if !ok {
			return nil
		}

		var payload wstypes.EchoPayload
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			fmt.Printf("waitEcho: unmarshal payload: %v\n", err)
			// to rearm
			return echoWsMsg{}
		}
		return echoWsMsg(payload)
	}
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, c := range m.comps {
		if cmd := c.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	cmds = append(cmds, waitNotification(), waitEcho())
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.relayout(msg.Width, msg.Height)
		return m, nil
	case types.ConnectedMsg:
		return m, tea.Batch(waitNotification(), waitEcho())
	case notificationMsg:
		return m, tea.Batch(
			func() tea.Msg { return components.NewResponseAppendMsg("notification: " + msg.Text) },
			waitNotification(),
		)
	case echoWsMsg:
		text := msg.Text
		// dispatch a new append message to the history component, and re-arm the echo wait.
		return m, tea.Batch(
			func() tea.Msg { return components.NewResponseAppendMsg("echo response: " + text) },
			waitEcho(),
		)
	case types.RoomSelectedMsg:
		roomID := msg.RoomID
		m.selectedRoomID = &roomID
		m.relayout(m.termW, m.termH)
		// Broadcast to all components (rooms, members).
		var cmds []tea.Cmd
		for _, c := range m.comps {
			if cmd := c.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.comps[m.focusable[m.focusedComponent]].SetFocused(false)
			m.focusedComponent = (m.focusedComponent + 1) % len(m.focusable)
			m.comps[m.focusable[m.focusedComponent]].SetFocused(true)
			return m, nil
		default:
			return m, m.comps[m.focusable[m.focusedComponent]].Update(msg)
		}
	default:
		// Broadcast non-key messages to all components.
		var cmds []tea.Cmd
		for _, c := range m.comps {
			if cmd := c.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	}
}

func (m model) View() tea.View {
	var topRightParts []string
	topRightParts = append(topRightParts, m.rooms.Render())
	if m.selectedRoomID != nil {
		topRightParts = append(topRightParts, m.members.Render())
	}
	topRight := lipgloss.JoinHorizontal(lipgloss.Top, topRightParts...)
	rightCol := lipgloss.JoinVertical(lipgloss.Left, topRight, m.status.Render())
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, m.history.Render(), rightCol)
	s := topRow + "\n" + m.command.Render()

	footerText := ""
	rem := types.ShellWrapWidth - len([]rune(footerText))
	s += "\n" + styles.HelpStyle.Render(footerText+fmt.Sprintf("%*s", rem, " "))

	return tea.NewView(styles.AppStyle.Render(s))
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
