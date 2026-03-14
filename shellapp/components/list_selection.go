package components

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/srschreiber/nito/shellapp/styles"
)

type ModelRow struct {
	Y      int
	Width  int
	Height int
}

type RowClickMsg struct{ Row int }
type RowHoverMsg struct{ Row int }

type Component interface {
	ComputeLayout(yOffset int) ([]ModelRow, int)
	Update(msg tea.Msg) tea.Cmd
	Render() string
}

type ListSelectionComponent struct {
	Title               string
	Choices             []string
	Selected            map[int]struct{}
	FocusedElementIndex int
}

func NewListSelectionComponent(title string, choices []string) *ListSelectionComponent {
	return &ListSelectionComponent{
		Title:    title,
		Choices:  choices,
		Selected: make(map[int]struct{}),
	}
}

func (l *ListSelectionComponent) toggle(row int) {
	if _, ok := l.Selected[row]; ok {
		delete(l.Selected, row)
	} else {
		l.Selected[row] = struct{}{}
	}
}

func (l *ListSelectionComponent) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if l.FocusedElementIndex > 0 {
				l.FocusedElementIndex--
			}
		case "down", "j":
			if l.FocusedElementIndex < len(l.Choices)-1 {
				l.FocusedElementIndex++
			}
		case "enter", "space":
			l.toggle(l.FocusedElementIndex)
		}
	case RowClickMsg:
		l.FocusedElementIndex = msg.Row
		l.toggle(msg.Row)
	case RowHoverMsg:
		l.FocusedElementIndex = msg.Row
	}
	return nil
}

func (l *ListSelectionComponent) Render() string {
	s := styles.TitleStyle.Render(l.Title) + "\n"
	for i, choice := range l.Choices {
		cursor := " "
		if l.FocusedElementIndex == i {
			cursor = styles.CursorStyle.Render("›")
		}

		checked := " "
		if _, ok := l.Selected[i]; ok {
			checked = styles.SelectedStyle.Render("✓")
			choice = styles.SelectedStyle.Render(choice)
		}

		row := fmt.Sprintf("%s [%s] %s", cursor, checked, choice)
		s += styles.ItemStyle.Render(row) + "\n"
	}
	return s
}

func (l *ListSelectionComponent) ComputeLayout(yOffset int) ([]ModelRow, int) {
	yOffset += lipgloss.Height(styles.TitleStyle.Render(l.Title) + "\n")

	rows := make([]ModelRow, 0, len(l.Choices))
	for i, choice := range l.Choices {
		checked := " "
		if _, ok := l.Selected[i]; ok {
			checked = styles.SelectedStyle.Render("✓")
			choice = styles.SelectedStyle.Render(choice)
		}

		rowText := fmt.Sprintf("  [%s] %s", checked, choice)
		renderedRow := styles.ItemStyle.Render(rowText)

		rows = append(rows, ModelRow{
			Y:      yOffset,
			Width:  lipgloss.Width(renderedRow),
			Height: lipgloss.Height(renderedRow),
		})

		yOffset += lipgloss.Height(renderedRow)
	}

	return rows, yOffset
}
