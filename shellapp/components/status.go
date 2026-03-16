package components

import (
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/srschreiber/nito/shellapp/styles"
	"github.com/srschreiber/nito/shellapp/types"
)

type StatusComponent struct {
	connected bool
	brokerURL string
	focused   bool
	width     int
	height    int
}

func NewStatusComponent(width, height int) *StatusComponent {
	return &StatusComponent{width: width, height: height}
}

func (s *StatusComponent) Init() tea.Cmd { return nil }

func (s *StatusComponent) SetFocused(focused bool) {
	s.focused = focused
}

func (s *StatusComponent) Update(msg tea.Msg) tea.Cmd {
	if m, ok := msg.(types.ConnectionStatusMsg); ok {
		s.connected = m.Connected
		s.brokerURL = m.BrokerURL
	}
	return nil
}

func (s *StatusComponent) Render() string {
	label := styles.StatusLabelStyle.Render("STATUS")

	var statusLine string
	if s.connected {
		statusLine = styles.StatusConnectedStyle.Render("● Connected") +
			"\n" + styles.Grey.Render("  "+s.brokerURL)
	} else {
		statusLine = styles.StatusDisconnectedStyle.Render("● Disconnected")
	}

	body := label + "\n" + statusLine

	borderColor := lipgloss.Color("238")
	if s.focused {
		borderColor = lipgloss.Color("213")
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(s.width).
		Height(s.height)
	return style.Render(body)
}
