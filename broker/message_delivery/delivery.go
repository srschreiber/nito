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

package message_delivery

import (
	"context"
	"log"

	wstypes "github.com/srschreiber/nito/websocket_types"
)
import db "github.com/srschreiber/nito/broker/database"

// InFlightMessageWriter collects messages that need to be inserted into the database, allowing
// routing to user's specific channels to make sure that database insertions will not block delivery.
// since there is only one broker, this will also guarantee delivery order.
type InFlightMessageWriter struct {
	outboundMessageChan chan wstypes.RoomMessagePayload
	conn                db.Conn
	ctx                 context.Context
}

func NewOutboundRoomMessages(ctx context.Context, conn db.Conn) *InFlightMessageWriter {
	return &InFlightMessageWriter{
		outboundMessageChan: make(chan wstypes.RoomMessagePayload, 250),
		conn:                conn,
		ctx:                 ctx,
	}
}

func (o *InFlightMessageWriter) Enqueue(payload wstypes.RoomMessagePayload) {
	// non-blocking send to the channel, dropping messages if the channel is full to avoid blocking delivery
	select {
	case o.outboundMessageChan <- payload:
	default:
		log.Println("outbound message channel is full")
	}
}

func (o *InFlightMessageWriter) Start() {
	// starts a goroutine to read from the channel and insert messages into the database
	go func() {
		for {
			select {
			case payload := <-o.outboundMessageChan:
				if err := db.InsertRoomMessage(o.ctx, o.conn, payload); err != nil {
					log.Printf("failed to insert room message: %v", err)
				}
			case <-o.ctx.Done():
				return
			}
		}
	}()
}
