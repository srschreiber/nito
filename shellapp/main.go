package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/srschreiber/nito/shellapp/components"
	"github.com/srschreiber/nito/shellapp/styles"
)

type model struct {
	width            int
	height           int
	focusedComponent int
	comps            []components.Component
}

func initialModel() model {
	list := components.NewListSelectionComponent(
		"What should we buy at the market?",
		[]string{"Buy carrots", "Buy celery", "Buy kohlrabi"},
	)
	list2 := components.NewListSelectionComponent(
		"What should we buy at the market?2",
		[]string{"Buy carrots2", "Buy celery2", "Buy kohlrabi2"},
	)
	m := model{
		comps: []components.Component{list, list2},
	}
	m.comps[0].SetFocused(true)
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.comps[m.focusedComponent].SetFocused(false)
			m.focusedComponent = (m.focusedComponent + 1) % len(m.comps)
			m.comps[m.focusedComponent].SetFocused(true)
		default:
			m.comps[m.focusedComponent].Update(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m model) View() tea.View {
	s := ""
	for _, c := range m.comps {
		s += c.Render() + "\n"
	}
	s += styles.HelpStyle.Render("tab focus • j/k or arrows navigate • space/enter select • q quit")

	return tea.NewView(styles.AppStyle.Render(styles.BoxStyle.Render(s)))
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
