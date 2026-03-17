package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/srschreiber/nito/broker/types"
	brokerws "github.com/srschreiber/nito/broker/websocket"
)

var addr = flag.String("addr", "localhost:7070", "http service address")

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

	broker := brokerws.NewBroker(*addr)
	ctx := context.Background()

	http.HandleFunc("/api/v0/ping", withValidation(ping))
	http.HandleFunc("/api/v0/register", withValidation(func(w http.ResponseWriter, r *http.Request, req types.RegisterRequest) {
		broker.RegisterUser(req.UserID)
		w.WriteHeader(http.StatusOK)
	}))
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		broker.WsConnect(ctx, w, r)
	})

	log.Printf("broker listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func ping(w http.ResponseWriter, _ *http.Request, req types.PingRequest) {
	resp := types.PingResponse{Message: req.Message}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
