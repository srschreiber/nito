package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

type modelRow struct {
	y      int
	width  int
	height int
}

type component interface {
	computeLayout(yOffset int) ([]modelRow, int)
}

type listSelectionComponent struct {
	title    string
	choices  []string
	selected map[int]struct{}
}

func (l *listSelectionComponent) toggle(row int) {
	if _, ok := l.selected[row]; ok {
		delete(l.selected, row)
	} else {
		l.selected[row] = struct{}{}
	}
}

func (l *listSelectionComponent) computeLayout(yOffset int) ([]modelRow, int) {
	yOffset += lipgloss.Height(titleStyle.Render(l.title))

	rows := make([]modelRow, 0, len(l.choices))
	for i, choice := range l.choices {
		checked := " "
		if _, ok := l.selected[i]; ok {
			checked = selectedStyle.Render("✓")
			choice = selectedStyle.Render(choice)
		}

		rowText := fmt.Sprintf("  [%s] %s", checked, choice)
		renderedRow := itemStyle.Render(rowText)

		rows = append(rows, modelRow{
			y:      yOffset,
			width:  lipgloss.Width(renderedRow),
			height: lipgloss.Height(renderedRow),
		})

		yOffset += lipgloss.Height(renderedRow)
	}

	return rows, yOffset
}

type model struct {
	width      int
	height     int
	cursorRow  int
	list       *listSelectionComponent
	components []component
}

// computeRowLayout calculates the position and size of each row based on the current choices and styles. This is used for mouse interaction to determine which row is being clicked or hovered over.
func (m model) computeRowLayout() []modelRow {
	yOffset := boxStyle.GetBorderTopSize() + boxStyle.GetPaddingTop()

	var rows []modelRow
	for _, c := range m.components {
		var componentRows []modelRow
		componentRows, yOffset = c.computeLayout(yOffset)
		rows = append(rows, componentRows...)
	}
	return rows
}

func initialModel() model {
	list := &listSelectionComponent{
		title:    "What should we buy at the market?",
		choices:  []string{"Buy carrots", "Buy celery", "Buy kohlrabi"},
		selected: make(map[int]struct{}),
	}
	return model{
		list:       list,
		components: []component{list},
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
			if m.cursorRow < len(m.list.choices)-1 {
				m.cursorRow++
			}
		case "enter", "space":
			m.list.toggle(m.cursorRow)
		}

	case tea.MouseClickMsg:
		mouse := msg.Mouse()

		if mouse.Button == tea.MouseNone {
			break
		}

		rows := m.computeRowLayout()
		for i, r := range rows {
			if mouse.Y >= r.y && mouse.Y < r.y+r.height {
				m.cursorRow = i
				m.list.toggle(i)
				break
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.MouseMotionMsg:
		mouse := msg.Mouse()

		rows := m.computeRowLayout()
		for i, r := range rows {
			if mouse.Y >= r.y && mouse.Y < r.y+r.height {
				m.cursorRow = i
				break
			}
		}
	}

	return m, nil
}

var (
	appStyle = lipgloss.NewStyle().
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2)
)

func (m model) View() tea.View {
	s := titleStyle.Render(m.list.title) + "\n"

	for i, choice := range m.list.choices {
		cursor := " "
		if m.cursorRow == i {
			cursor = cursorStyle.Render("›")
		}

		checked := " "
		if _, ok := m.list.selected[i]; ok {
			checked = selectedStyle.Render("✓")
			choice = selectedStyle.Render(choice)
		}

		row := fmt.Sprintf("%s [%s] %s", cursor, checked, choice)
		s += itemStyle.Render(row) + "\n"
	}

	s += helpStyle.Render("j/k or arrows • click row • space/enter select • q quit")

	v := tea.NewView(appStyle.Render(boxStyle.Render(s)))
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
