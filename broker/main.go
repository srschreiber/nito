package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/srschreiber/nito/broker/database"
	"github.com/srschreiber/nito/broker/types"
	brokerws "github.com/srschreiber/nito/broker/websocket"
)

var configPath = flag.String("config", "broker/config.yml", "path to config file")
var migrateOnly = flag.Bool("migrate-only", false, "run migrations and exit")

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

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Single connection used only for sequential migration at startup.
	conn, err := database.NewPostgres(cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	if err := database.RunMigrations(conn); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	conn.Close(context.Background())

	if *migrateOnly {
		return
	}

	// Pool used by the broker for concurrent request handling.
	pool, err := database.NewPool(cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)
	if err != nil {
		log.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	broker := brokerws.NewBroker(cfg.Broker.Addr, pool)
	ctx := context.Background()

	http.HandleFunc("/api/v0/ping", withValidation(ping))
	http.HandleFunc("/api/v0/register", withValidation(func(w http.ResponseWriter, r *http.Request, req types.RegisterRequest) {
		resp, err := broker.RegisterUser(r.Context(), req.Username, req.PublicKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		broker.WsConnect(ctx, w, r)
	})

	log.Printf("broker listening on %s", cfg.Broker.Addr)
	log.Fatal(http.ListenAndServe(cfg.Broker.Addr, nil))
}

func ping(w http.ResponseWriter, _ *http.Request, req types.PingRequest) {
	resp := types.PingResponse{Message: req.Message}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
