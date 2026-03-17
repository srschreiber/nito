package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
	"github.com/srschreiber/nito/broker/types"
)

var addr = flag.String("addr", "localhost:7070", "http service address")

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var validate = validator.New(validator.WithRequiredStructEnabled())

func withValidation[Req any](handler func(w http.ResponseWriter, r *http.Request, req Req)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := validate.Struct(req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		handler(w, r, req)
	}
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	registerAPIs()
	registerWebsocketEndpoints()
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func registerAPIs() {
	http.HandleFunc("/api/v0/ping", withValidation(ping))
}

func registerWebsocketEndpoints() {
	http.HandleFunc("/ws", wsConnect)
}

func ping(w http.ResponseWriter, _ *http.Request, req types.PingRequest) {
	resp := types.PingResponse{Message: req.Message}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// TODO:
// each user will have a unique websocket with a read and write loop
// for example
//
//	type Client struct {
//		UserID string
//		Conn   *websocket.Conn
//		Send   chan []byte
//	}
//
//	type Broker struct {
//		mu      sync.RWMutex
//		clients map[string]*Client
//	}
//
//	func NewBroker() *Broker {
//		return &Broker{
//			clients: make(map[string]*Client),
//		}
//	}
//
//	func (b *Broker) AddClient(c *Client) {
//		b.mu.Lock()
//		defer b.mu.Unlock()
//		b.clients[c.UserID] = c
//	}
//
//	func (b *Broker) RemoveClient(userID string) {
//		b.mu.Lock()
//		defer b.mu.Unlock()
//		delete(b.clients, userID)
//	}
//
//	func (b *Broker) GetClient(userID string) (*Client, bool) {
//		b.mu.RLock()
//		defer b.mu.RUnlock()
//		c, ok := b.clients[userID]
//		return c, ok
//	}
//
//	func writeLoop(client *Client) {
//		defer client.Conn.Close()
//
//		for msg := range client.Send {
//			if err := client.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
//				return
//			}
//		}
//	}
//
// then, wsConnect will get the userID from the request. Of course later we will require a signature using the user's private key, but for development it doesnt matter yet
func wsConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}
	defer conn.Close()
	log.Println("client connected:", r.RemoteAddr)

	// Keep the connection alive until the client disconnects.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Println("client disconnected:", r.RemoteAddr)
			return
		}
	}
	// todo:
	//func wsConnect(broker *Broker, w http.ResponseWriter, r *http.Request) {
	//	userID := r.URL.Query().Get("user_id") // real app should authenticate properly
	//	if userID == "" {
	//		http.Error(w, "missing user_id", http.StatusUnauthorized)
	//		return
	//	}
	//
	//	conn, err := upgrader.Upgrade(w, r, nil)
	//	if err != nil {
	//		log.Println("upgrade error:", err)
	//		return
	//	}
	//
	//	client := &Client{
	//		UserID: userID,
	//		Conn:   conn,
	//		Send:   make(chan []byte, 32),
	//	}
	//
	//	broker.AddClient(client)
	//	log.Println("client connected:", userID)
	//
	//	go writeLoop(client)
	//
	//	readLoop(broker, client)
	//}
}
