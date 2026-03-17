package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/srschreiber/nito/broker/types"
)

type Broker struct {
	Address   string `description:"Websocket broker address"`
	upgrader  websocket.Upgrader
	clientMap map[string]*Client `description:"Map of connected clients by user ID"`
}

type Client struct {
	UserID     string          `description:"Unique identifier for the client"`
	send       chan []byte     `description:"Channel for sending messages to the client"`
	connection *websocket.Conn `description:"Websocket connection"`
}

func NewBroker(address string) *Broker {
	return &Broker{
		Address:   address,
		clientMap: make(map[string]*Client),
		upgrader:  websocket.Upgrader{},
	}
}

func (b *Broker) AddClient(client *Client) {
	if b.clientMap[client.UserID] != nil {
		log.Printf("warning: client with userID %s already exists, will reuse existing", client.UserID)
		return
	}

	b.clientMap[client.UserID] = client
}

func (b *Broker) removeClient(client *Client) {
	delete(b.clientMap, client.UserID)
	log.Printf("client %s disconnected", client.UserID)
}

func (b *Broker) WsConnect(ctx context.Context, broker *Broker, w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "missing user_id", http.StatusUnauthorized)
		return
	}

	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second)) // set initial read deadline`
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(30 * time.Second)) // extend read deadline on pong
	})

	client := &Client{
		UserID:     userID,
		connection: conn,
		send:       make(chan []byte, 32),
	}

	broker.AddClient(client)
	log.Println("client connected:", userID)

	go b.writeLoop(ctx, client)

	if err = b.readLoop(ctx, broker, client); err != nil {
		log.Println("read loop error for client", userID, ":", err)
	}

	b.removeClient(client)
}

func (b *Broker) writeLoop(ctx context.Context, client *Client) {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("client disconnected:", client.UserID)
			return
		case <-pingTicker.C:
			if err := client.connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				_ = client.connection.Close()
				return
			}
		case msg, ok := <-client.send:
			if !ok {
				log.Println("client disconnected:", client.UserID)
				_ = client.connection.Close()
				return
			}
			if err := client.connection.WriteMessage(websocket.TextMessage, msg); err != nil {
				_ = client.connection.Close()
				return
			}
		}
	}
}

func (b *Broker) readLoop(ctx context.Context, broker *Broker, client *Client) error {
	defer client.connection.Close()

	go func() {
		<-ctx.Done()
		_ = client.connection.Close()
	}()

	for {

		messageType, data, err := client.connection.ReadMessage()
		if err != nil {
			return err
		}

		switch messageType {
		case websocket.TextMessage:
			var msg types.WebsocketMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			//if err := c.handleMessage(msg); err != nil {
			//	continue
			//}
		}
	}
}
