package components

import (
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/srschreiber/nito/shellapp/styles"
)

// HintsComponent renders context-sensitive keybinding hints for the focused component.
type HintsComponent struct {
	focusedComp int // comps index: 0=history, 1=rooms, 4=command
	chatMode    bool
	width       int
	height      int
}

func NewHintsComponent(width, height int) *HintsComponent {
	return &HintsComponent{width: width, height: height, focusedComp: 4}
}

func (h *HintsComponent) SetSize(width, height int) {
	h.width = width
	h.height = height
}

func (h *HintsComponent) SetFocused(_ bool) {}

func (h *HintsComponent) SetFocusedComp(idx int) {
	h.focusedComp = idx
}

func (h *HintsComponent) Init() tea.Cmd { return nil }

func (h *HintsComponent) Update(msg tea.Msg) tea.Cmd {
	if m, ok := msg.(ModeChangedMsg); ok {
		h.chatMode = m.ChatMode
	}
	return nil
}

func (h *HintsComponent) Render() string {
	k := lipgloss.NewStyle().Foreground(lipgloss.Color("222")).Bold(true)
	d := styles.Grey
	sep := d.Render("  •  ")

	var lines []string
	switch h.focusedComp {
	case 0: // history
		lines = []string{
			k.Render("↑/↓") + d.Render(" / ") + k.Render("ctrl+p/n") + d.Render("  scroll"),
			k.Render("jump -L <n>") + d.Render("  go to line"),
		}
	case 1: // rooms
		lines = []string{
			k.Render("↑/↓") + d.Render(" / ") + k.Render("ctrl+p/n") + d.Render("  navigate"),
			k.Render("enter") + d.Render("  select room"),
		}
	default: // command (idx 4)
		nav := k.Render("ctrl+b/f") + d.Render(" move") + sep + k.Render("ctrl+a/e") + d.Render(" home/end")
		del := k.Render("ctrl+k") + d.Render(" del to end")
		delSingle := k.Render("ctrl+d") + d.Render(" del char")
		if h.chatMode {
			lines = []string{
				nav,
				delSingle + sep + del,
				k.Render("shift+enter") + d.Render(" newline") + sep + k.Render("enter") + d.Render(" send"),
			}
		} else {
			lines = []string{
				k.Render("↑/↓") + d.Render(" / ") + k.Render("ctrl+p/n") + d.Render("  history"),
				nav,
				delSingle + sep + del,
				k.Render("enter") + d.Render("  run command"),
			}
		}
	}

	body := styles.SectionTitleStyle.Render("Keys") + "\n"
	for _, l := range lines {
		body += l + "\n"
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Width(h.width).
		Height(h.height).
		Render(body)
}
