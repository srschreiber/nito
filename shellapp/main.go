package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/srschreiber/nito/shellapp/components"
	"github.com/srschreiber/nito/shellapp/styles"
)

type model struct {
	width     int
	height    int
	cursorRow int
	list      *components.ListSelectionComponent
	comps     []components.Component
}

// computeComponentRowLayout calculates the position and size of each row based on the current choices and styles. This is used for mouse interaction to determine which row is being clicked or hovered over.
func (m model) computeComponentRowLayout() []components.ModelRow {
	yOffset := 4

	var rows []components.ModelRow
	for _, c := range m.comps {
		var componentRows []components.ModelRow
		componentRows, yOffset = c.ComputeLayout(yOffset)
		rows = append(rows, componentRows...)
	}
	return rows
}

func initialModel() model {
	list := components.NewListSelectionComponent(
		"What should we buy at the market?",
		[]string{"Buy carrots", "Buy celery", "Buy kohlrabi"},
	)
	return model{
		list:  list,
		comps: []components.Component{list},
	}
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
		case "up", "k":
			if m.cursorRow > 0 {
				m.cursorRow--
			}
		case "down", "j":
			if m.cursorRow < len(m.list.Choices)-1 {
				m.cursorRow++
			}
		}
		for _, c := range m.comps {
			c.Update(msg)
		}

	case tea.MouseClickMsg:
		mouse := msg.Mouse()

		if mouse.Button == tea.MouseNone {
			break
		}

		rows := m.computeComponentRowLayout()
		for i, r := range rows {
			if mouse.Y >= r.Y && mouse.Y < r.Y+r.Height {
				m.cursorRow = i
				for _, c := range m.comps {
					c.Update(components.RowClickMsg{Row: i})
				}
				break
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.MouseMotionMsg:
		mouse := msg.Mouse()

		rows := m.computeComponentRowLayout()
		for i, r := range rows {
			if mouse.Y >= r.Y && mouse.Y < r.Y+r.Height {
				m.cursorRow = i
				for _, c := range m.comps {
					c.Update(components.RowHoverMsg{Row: i})
				}
				break
			}
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	s := ""
	for _, c := range m.comps {
		s += c.Render()
	}
	s += styles.HelpStyle.Render("j/k or arrows • click row • space/enter select • q quit")

	v := tea.NewView(styles.AppStyle.Render(styles.BoxStyle.Render(s)))
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
