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

package types

// ConnectionStatusMsg is broadcast after any command that may change connection state.
type ConnectionStatusMsg struct {
	Connected bool
	BrokerURL string
	UserID    string
}

// ConnectedMsg is sent once after a successful connect, to re-arm WS listeners.
type ConnectedMsg struct{}

// MembersUpdatedMsg is sent when the broker pushes a "members_updated" RPC,
// signalling that room member online/offline status should be refetched.
type MembersUpdatedMsg struct{}
