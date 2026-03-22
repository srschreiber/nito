package message_delivery

import (
	"context"
	"log"

	wstypes "github.com/srschreiber/nito/websocket_types"
)
import db "github.com/srschreiber/nito/broker/database"

// OutboundRoomMessages collects messages that need to be inserted into the database, allowing
// routing to user's specific channels to make sure that database insertions will not block delivery.
// since there is only one broker, this will also guarantee delivery order.
type OutboundRoomMessages struct {
	outboundMessageChan chan wstypes.RoomMessagePayload
	conn                db.Conn
	ctx                 context.Context
}

func NewOutboundRoomMessages(ctx context.Context, conn db.Conn) *OutboundRoomMessages {
	return &OutboundRoomMessages{
		outboundMessageChan: make(chan wstypes.RoomMessagePayload, 250),
		conn:                conn,
		ctx:                 ctx,
	}
}

func (o *OutboundRoomMessages) Enqueue(payload wstypes.RoomMessagePayload) {
	// non-blocking send to the channel, dropping messages if the channel is full to avoid blocking delivery
	select {
	case o.outboundMessageChan <- payload:
	default:
		log.Println("outbound message channel is full")
	}
}

func (o *OutboundRoomMessages) Start() {
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
