package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/srschreiber/nito/shellapp/components"
	"github.com/srschreiber/nito/shellapp/styles"
	"github.com/srschreiber/nito/shellapp/types"
)

// Layout constants (lipgloss content dimensions, excluding borders/padding).
const (
	histWidth  = 55
	histHeight = 18
	statWidth  = 22
	// statHeight matches histHeight for a uniform top row
	cmdWidth = histWidth + statWidth + 8 // approx combined box overhead
)

type model struct {
	width            int
	height           int
	focusedComponent int
	history          *components.ConversationHistory
	status           *components.StatusComponent
	command          *components.CommandComponent
	comps            []components.Component
	// focusable holds the comps indices that participate in tab cycling
	focusable []int
}

func initialModel() model {
	history := components.NewConversationHistory(histWidth, histHeight)
	status := components.NewStatusComponent(statWidth, histHeight)
	command := components.NewCommandComponent(cmdWidth)

	// comps: 0=history, 1=status (display-only), 2=command
	m := model{
		history:          history,
		status:           status,
		command:          command,
		comps:            []components.Component{history, status, command},
		focusable:        []int{0, 2},
		focusedComponent: 1, // index into focusable → comps[2] = command
	}
	m.comps[m.focusable[m.focusedComponent]].SetFocused(true)
	return m
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, c := range m.comps {
		if cmd := c.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
		// Broadcast non-key messages to all components
		// (cursor blink, AppendHistoryMsg, ClearHistoryMsg, ConnectionStatusMsg, etc.)
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
	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.history.Render(),
		m.status.Render(),
	)
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
