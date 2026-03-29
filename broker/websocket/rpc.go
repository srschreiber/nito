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

package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/srschreiber/nito/broker/auth"
	"github.com/srschreiber/nito/broker/database"
	wstypes "github.com/srschreiber/nito/shared/websocket_types"
)

func (b *Broker) handleRPC(client *Client, msg wstypes.ToBrokerWsMessage) {
	switch msg.RPCName {
	case wstypes.RPCEcho:
		b.handleEcho(client, msg)
	case wstypes.RPCRoomMessage:
		if b.sendRoomMessage(client, msg) != nil {
			log.Printf("error handling room_message from %s", client.Session.UserID)
		}
	default:
		log.Printf("unknown RPC %q from %s", msg.RPCName, client.Session.UserID)
	}
}

func (b *Broker) verifyRPCSignature(client *Client, msg wstypes.ToBrokerWsMessage) error {
	pubKey, err := database.GetUserPublicKeyByUsername(context.Background(), b.DB, client.Session.Username)
	if err != nil || pubKey == nil {
		return fmt.Errorf("public key not found for user %s", client.Session.Username)
	}
	signed := client.Session.Username + ":" + msg.RPCName
	return auth.VerifySignature(*pubKey, signed, msg.Signature)
}

func (b *Broker) handleEcho(client *Client, msg wstypes.ToBrokerWsMessage) {
	var payload wstypes.EchoPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("echo: bad payload from %s: %v", client.Session.UserID, err)
		return
	}

	runes := []rune(payload.Text)
	if len(runes) > wstypes.EchoMaxChars {
		runes = runes[:wstypes.EchoMaxChars]
	}
	payload.Text = string(runes)

	respPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("echo: marshal error: %v", err)
		return
	}

	response := wstypes.ToClientWsMessage{
		RPCName:   wstypes.RPCEcho,
		RequestID: msg.RequestID,
		UserID:    client.Session.UserID,
		Nonce:     msg.Nonce,
		Timestamp: time.Now().Unix(),
		Payload:   respPayload,
	}
	data, err := json.Marshal(response)
	if err != nil {
		log.Printf("echo: marshal response error: %v", err)
		return
	}
	client.send <- data
}
