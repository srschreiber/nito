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
	http.HandleFunc("/ping", withValidation(ping))
	http.HandleFunc("/ws/ping", wsPing)
	http.HandleFunc("/ws", wsConnect)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func ping(w http.ResponseWriter, _ *http.Request, req types.PingRequest) {
	resp := types.PingResponse{Message: req.Message}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

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
}

func wsPing(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}
	defer conn.Close()

	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Println("read error:", err)
		return
	}

	var req types.PingRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid json"}`))
		return
	}
	if err := validate.Struct(req); err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"`+err.Error()+`"}`))
		return
	}

	resp := types.PingResponse{Message: req.Message}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, respBytes)
}
