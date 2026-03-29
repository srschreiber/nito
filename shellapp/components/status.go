// Copyright 2026 Sam Schreiber
//
// This file is part of nito.
//
// nito is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// nito is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with nito. If not, see <https://www.gnu.org/licenses/>.

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
	userID    string
	focused   bool
	width     int
	height    int
}

func NewStatusComponent(width, height int) *StatusComponent {
	return &StatusComponent{width: width, height: height}
}

func (s *StatusComponent) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *StatusComponent) Init() tea.Cmd { return nil }

func (s *StatusComponent) SetFocused(focused bool) {
	s.focused = focused
}

func (s *StatusComponent) Update(msg tea.Msg) tea.Cmd {
	if m, ok := msg.(types.ConnectionStatusMsg); ok {
		s.connected = m.Connected
		s.brokerURL = m.BrokerURL
		s.userID = m.UserID
	}
	return nil
}

func (s *StatusComponent) Render() string {
	label := styles.SectionTitleStyle.Render("Status")

	var statusLine string
	if s.connected {
		statusLine = styles.StatusConnectedStyle.Render("● Connected") +
			"\n" + styles.Grey.Render("  "+s.brokerURL) +
			"\n" + styles.Grey.Render("  user: "+s.userID)
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
