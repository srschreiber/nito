package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/srschreiber/nito/broker/types"
)

type Session struct {
	UserID string
}

type Client struct {
	Session    Session
	send       chan []byte
	connection *websocket.Conn
}

type Broker struct {
	Address   string `description:"Websocket broker address"`
	upgrader  websocket.Upgrader
	mu        sync.RWMutex
	clientMap map[string]*Client `description:"Map of connected clients by user ID"`
	userStore map[string]bool    `description:"In-memory set of registered user IDs"`
}

func NewBroker(address string) *Broker {
	return &Broker{
		Address:   address,
		clientMap: make(map[string]*Client),
		userStore: make(map[string]bool),
		upgrader:  websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
}

// RegisterUser adds a user to the in-memory store.
func (b *Broker) RegisterUser(userID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.userStore[userID] = true
	log.Printf("registered user: %s", userID)
}

func (b *Broker) isRegistered(userID string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.userStore[userID]
}

func (b *Broker) addClient(client *Client) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.clientMap[client.Session.UserID] != nil {
		log.Printf("warning: client with userID %s already connected", client.Session.UserID)
		return false
	}
	b.clientMap[client.Session.UserID] = client
	return true
}

func (b *Broker) removeClient(client *Client) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clientMap, client.Session.UserID)
	log.Printf("client %s disconnected", client.Session.UserID)
}

func (b *Broker) WsConnect(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "missing user_id", http.StatusUnauthorized)
		return
	}

	if !b.isRegistered(userID) {
		http.Error(w, "user not registered: call POST /api/v0/register first", http.StatusUnauthorized)
		return
	}

	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	})

	client := &Client{
		Session:    Session{UserID: userID},
		connection: conn,
		send:       make(chan []byte, 32),
	}

	if !b.addClient(client) {
		http.Error(w, "user already connected", http.StatusConflict)
		_ = conn.Close()
		return
	}
	log.Println("client connected:", userID)

	go b.writeLoop(ctx, client)

	if err = b.readLoop(ctx, client); err != nil {
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
			return
		case <-pingTicker.C:
			if err := client.connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				_ = client.connection.Close()
				return
			}
		case msg, ok := <-client.send:
			if !ok {
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

func (b *Broker) readLoop(ctx context.Context, client *Client) error {
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
				log.Printf("invalid message from %s: %v", client.Session.UserID, err)
				continue
			}
			log.Printf("message from %s: rpc=%s requestId=%s", client.Session.UserID, msg.RPCName, msg.RequestID)
			b.handleRPC(client, msg)
		}
	}
}

func (b *Broker) handleRPC(client *Client, msg types.WebsocketMessage) {
	switch msg.RPCName {
	case "echo":
		b.handleEcho(client, msg)
	default:
		log.Printf("unknown RPC %q from %s", msg.RPCName, client.Session.UserID)
	}
}

func (b *Broker) handleEcho(client *Client, msg types.WebsocketMessage) {
	var payload types.EchoPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("echo: bad payload from %s: %v", client.Session.UserID, err)
		return
	}

	// Chop message to max allowed characters.
	runes := []rune(payload.Text)
	if len(runes) > types.EchoMaxChars {
		runes = runes[:types.EchoMaxChars]
	}
	payload.Text = string(runes)

	respPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("echo: marshal error: %v", err)
		return
	}

	response := types.WebsocketMessage{
		RPCName:   "echo",
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
