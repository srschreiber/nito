package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	apitypes "github.com/srschreiber/nito/api_types"
	"github.com/srschreiber/nito/broker/auth"
	"github.com/srschreiber/nito/broker/database"
	dbtypes "github.com/srschreiber/nito/broker/database/types"
	"github.com/srschreiber/nito/broker/message_delivery"
	wstypes "github.com/srschreiber/nito/websocket_types"
)

type Session struct {
	UserID   string
	Username string
}

type Client struct {
	Session    Session
	send       chan []byte
	connection *websocket.Conn
}

type Broker struct {
	Address          string
	db               *pgxpool.Pool
	upgrader         websocket.Upgrader
	mu               sync.RWMutex
	clientMap        map[string]*Client
	inflightMessages *message_delivery.InFlightMessageWriter
}

// trySend attempts a non-blocking send to the client's send channel.
// If the channel is full the message is dropped and a warning is logged.
func (c *Client) trySend(data []byte) {
	select {
	case c.send <- data:
	default:
		log.Printf("send channel full for user %s — message dropped", c.Session.UserID)
	}
}

func (b *Broker) getClientForUserID(userID string) *Client {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.clientMap[userID]
}

func NewBroker(ctx context.Context, address string, db *pgxpool.Pool) *Broker {
	outbound := message_delivery.NewOutboundRoomMessages(ctx, db)
	outbound.Start()
	return &Broker{
		Address:          address,
		db:               db,
		clientMap:        make(map[string]*Client),
		upgrader:         websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		inflightMessages: outbound,
	}
}

// RegisterUser creates a new user in the DB, or returns the existing one if the username is taken.
func (b *Broker) RegisterUser(ctx context.Context, username, publicKey string) (*apitypes.RegisterResponse, error) {
	var existing dbtypes.User
	err := b.db.QueryRow(ctx,
		`SELECT id, username, public_key, updated_at, created_at FROM users WHERE username = $1`,
		username,
	).Scan(&existing.ID, &existing.Username, &existing.PublicKey, &existing.UpdatedAt, &existing.CreatedAt)
	if err == nil {
		return &apitypes.RegisterResponse{ID: existing.ID, Username: existing.Username, AlreadyRegistered: true}, nil
	}

	user, err := database.CreateUser(ctx, b.db, username, &publicKey)
	if err != nil {
		return nil, err
	}
	return &apitypes.RegisterResponse{ID: user.ID, Username: user.Username}, nil
}

// LookupUserIDByUsername resolves a username to its UUID. Returns "" if not found.
func (b *Broker) LookupUserIDByUsername(ctx context.Context, username string) string {
	var id string
	_ = b.db.QueryRow(ctx, `SELECT id FROM users WHERE username = $1`, username).Scan(&id)
	return id
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
	username := r.URL.Query().Get("user_id")
	if username == "" {
		http.Error(w, "missing user_id", http.StatusUnauthorized)
		return
	}

	userID := b.LookupUserIDByUsername(ctx, username)
	if userID == "" {
		http.Error(w, "user not registered: call POST /api/v0/register first", http.StatusUnauthorized)
		return
	}

	sigB64 := r.Header.Get("X-Signature")
	if sigB64 == "" {
		http.Error(w, "missing X-Signature header", http.StatusUnauthorized)
		return
	}
	pubKey, err := database.GetUserPublicKeyByUsername(ctx, b.db, username)
	if err != nil || pubKey == nil {
		http.Error(w, "user has no public key", http.StatusUnauthorized)
		return
	}
	if err := auth.VerifySignature(*pubKey, username+":/ws", sigB64); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	})

	client := &Client{
		Session:    Session{UserID: userID, Username: username},
		connection: conn,
		send:       make(chan []byte, 32),
	}

	if !b.addClient(client) {
		http.Error(w, "user already connected", http.StatusConflict)
		_ = conn.Close()
		return
	}
	log.Println("client connected:", userID)
	go b.notifyMembersUpdated(userID)

	go b.writeLoop(ctx, client)

	if err = b.readLoop(ctx, client); err != nil {
		log.Println("read loop error for client", userID, ":", err)
	}

	b.removeClient(client)
	go b.notifyMembersUpdated(userID)
}

func (b *Broker) writeLoop(ctx context.Context, client *Client) {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pingTicker.C:
			if err := client.connection.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
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
			var msg wstypes.ToBrokerWsMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Printf("invalid message from %s: %v", client.Session.UserID, err)
				continue
			}
			if err := b.verifyRPCSignature(client, msg); err != nil {
				log.Printf("signature error from %s: rpc=%s: %v", client.Session.UserID, msg.RPCName, err)
				continue
			}
			log.Printf("message from %s: rpc=%s requestId=%s", client.Session.UserID, msg.RPCName, msg.RequestID)
			b.handleRPC(client, msg)
		}
	}
}
