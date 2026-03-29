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

package commands

import (
	"errors"
	"fmt"

	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/voice"
)

func voiceJoinCmd() (string, error) {
	roomID := connection.GetSessionRoomID()
	if roomID == nil {
		return "", errors.New("voice-join: no room selected (use room-select first)")
	}
	if err := voice.Join(*roomID); err != nil {
		return "", fmt.Errorf("voice-join: %w", err)
	}
	return "joined voice in room " + *roomID, nil
}

func voiceLeaveCmd() (string, error) {
	roomID := connection.GetSessionRoomID()
	if roomID == nil {
		return "", errors.New("voice-leave: no room selected")
	}
	if err := voice.Leave(*roomID); err != nil {
		return "", fmt.Errorf("voice-leave: %w", err)
	}
	return "left voice in room " + *roomID, nil
}
