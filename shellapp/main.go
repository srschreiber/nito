package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/srschreiber/nito/shellapp/components"
	"github.com/srschreiber/nito/shellapp/styles"
	"github.com/srschreiber/nito/shellapp/types"
)

type model struct {
	width            int
	height           int
	focusedComponent int
	comps            []components.Component
}

func initialModel() model {
	// example
	//list := components.NewListSelectionComponent(
	//	"What should we buy at the market?",
	//	[]string{"Buy carrots", "Buy celery", "Buy kohlrabi"},
	//)

	command := components.NewCommandComponent()
	m := model{
		comps: []components.Component{command},
	}
	m.comps[0].SetFocused(true)
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
			m.comps[m.focusedComponent].SetFocused(false)
			m.focusedComponent = (m.focusedComponent + 1) % len(m.comps)
			m.comps[m.focusedComponent].SetFocused(true)
		default:
			return m, m.comps[m.focusedComponent].Update(msg)
		}
		//
		//case tea.WindowSizeMsg:
		//	//m.width = msg.Width
		//	//m.height = msg.Height
		//	//default:
		//	//	var cmds []tea.Cmd
		//	//	for _, c := range m.comps {
		//	//		if cmd := c.Update(msg); cmd != nil {
		//	//			cmds = append(cmds, cmd)
		//	//		}
		//	//	}
		//	//	return m, tea.Batch(cmds...)
	}

	return m, nil
}

func (m model) View() tea.View {
	s := ""
	for _, c := range m.comps {
		s += c.Render() + "\n"
	}
	footerText := "tab focus • j/k or arrows navigate • space/enter select"
	s += footerText
	rem := types.ShellWrapWidth - len(footerText)
	s += styles.HelpStyle.Render(fmt.Sprintf("%*s", rem, " "))
	s += styles.HelpStyle.Render("tab focus • j/k or arrows navigate • space/enter select")

	return tea.NewView(styles.AppStyle.Render(styles.BoxStyle.Render(s)))
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
