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

// Box overhead constants (lipgloss borders + padding).
// History/Status: RoundedBorder (all 4 sides) + Padding(0,1) → 4 wide, 2 tall.
// Command: ThickBorder (left only) + Padding(0,1) → 3 wide, 0 tall.
// AppStyle: Padding(1,2) → 4 wide, 2 tall.
const (
	histBoxOverheadW = 4
	histBoxOverheadH = 2
	statBoxOverheadW = 4
	cmdBoxOverheadW  = 3
	appPaddingW      = 4
)

// layout holds computed content dimensions for each component.
type layout struct {
	histW, histH int
	statW        int
	cmdW         int
}

// computeLayout derives component content dimensions from the terminal size.
// History box is 60% of terminal width and 80% of terminal height (visual box
// including borders). Status takes the remaining top-row width. Command spans
// the full usable width.
func computeLayout(termW, termH int) layout {
	if termW < 30 {
		termW = 30
	}
	if termH < 12 {
		termH = 12
	}

	usableW := termW - appPaddingW

	histBoxW := int(float64(termW) * 0.6)
	histBoxH := int(float64(termH) * 0.8)

	histW := histBoxW - histBoxOverheadW
	histH := histBoxH - histBoxOverheadH

	statBoxW := usableW - histBoxW
	statW := statBoxW - statBoxOverheadW

	cmdW := usableW - cmdBoxOverheadW

	if histW < 10 {
		histW = 10
	}
	if histH < 3 {
		histH = 3
	}
	if statW < 5 {
		statW = 5
	}
	if cmdW < 10 {
		cmdW = 10
	}

	return layout{histW: histW, histH: histH, statW: statW, cmdW: cmdW}
}

type model struct {
	history *components.ConversationHistory
	status  *components.StatusComponent
	command *components.CommandComponent
	comps   []components.Component
	// focusable holds the comps indices that participate in tab cycling
	focusable        []int
	focusedComponent int
}

func initialModel() model {
	l := computeLayout(120, 40) // reasonable default until WindowSizeMsg arrives
	history := components.NewConversationHistory(l.histW, l.histH)
	status := components.NewStatusComponent(l.statW, l.histH)
	command := components.NewCommandComponent(l.cmdW)

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

func (m *model) relayout(termW, termH int) {
	l := computeLayout(termW, termH)
	m.history.SetSize(l.histW, l.histH)
	m.status.SetSize(l.statW, l.histH)
	m.command.SetWidth(l.cmdW, l.histW)
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
	case tea.WindowSizeMsg:
		m.relayout(msg.Width, msg.Height)
		return m, nil
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
