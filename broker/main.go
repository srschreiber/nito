package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/srschreiber/nito/broker/auth"
	"github.com/srschreiber/nito/broker/database"
	"github.com/srschreiber/nito/broker/types"
	brokerws "github.com/srschreiber/nito/broker/websocket"
)

var configPath = flag.String("config", "broker/config.yml", "path to config file")
var migrateOnly = flag.Bool("migrate-only", false, "run migrations and exit")

var validate = validator.New(validator.WithRequiredStructEnabled())

// withSignature validates the X-Username and X-Signature headers on a request.
// The expected signed payload is "username:path" (e.g. "alice:/api/v0/rooms").
func withSignature(db *pgxpool.Pool, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.Header.Get("X-Username")
		sigB64 := r.Header.Get("X-Signature")
		if username == "" || sigB64 == "" {
			http.Error(w, "missing signature headers", http.StatusUnauthorized)
			return
		}
		pubKey, err := database.GetUserPublicKeyByUsername(r.Context(), db, username)
		if err != nil || pubKey == nil {
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}
		if err := auth.VerifySignature(*pubKey, username+":"+r.URL.Path, sigB64); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}
}

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
	http.HandleFunc("/api/v0/rooms", withSignature(pool, withValidation(func(w http.ResponseWriter, r *http.Request, req types.CreateRoomRequest) {
		userID := broker.LookupUserIDByUsername(r.Context(), req.UserID)
		if userID == "" {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		resp, err := broker.BrokerCreateRoom(r.Context(), userID, req.Name, req.EncryptedRoomKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})))
	http.HandleFunc("/api/v0/rooms/list", withSignature(pool, func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("user_id")
		if username == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}
		userID := broker.LookupUserIDByUsername(r.Context(), username)
		if userID == "" {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		rooms, err := broker.BrokerListUserRooms(r.Context(), userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(types.ListRoomsResponse{Rooms: rooms})
	}))
	http.HandleFunc("/api/v0/rooms/invite", withSignature(pool, withValidation(func(w http.ResponseWriter, r *http.Request, req types.InviteUserRequest) {
		username := r.Header.Get("X-Username")
		inviterID := broker.LookupUserIDByUsername(r.Context(), username)
		if inviterID == "" {
			http.Error(w, "inviter not found", http.StatusNotFound)
			return
		}
		resp, err := broker.BrokerInviteUser(r.Context(), req.RoomID, inviterID, req.InvitedUsername, req.EncryptedRoomKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})))
	http.HandleFunc("/api/v0/rooms/members", withSignature(pool, func(w http.ResponseWriter, r *http.Request) {
		roomID := r.URL.Query().Get("room_id")
		if roomID == "" {
			http.Error(w, "missing room_id", http.StatusBadRequest)
			return
		}
		members, err := broker.BrokerListRoomMembers(r.Context(), roomID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(types.ListRoomMembersResponse{Members: members})
	}))
	http.HandleFunc("/api/v0/rooms/key", withSignature(pool, func(w http.ResponseWriter, r *http.Request) {
		username := r.Header.Get("X-Username")
		userID := broker.LookupUserIDByUsername(r.Context(), username)
		if userID == "" {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		roomID := r.URL.Query().Get("room_id")
		if roomID == "" {
			http.Error(w, "missing room_id", http.StatusBadRequest)
			return
		}
		key, err := broker.BrokerGetRoomKey(r.Context(), userID, roomID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(types.GetRoomKeyResponse{EncryptedRoomKey: key})
	}))
	http.HandleFunc("/api/v0/rooms/invites", withSignature(pool, func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("user_id")
		if username == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}
		userID := broker.LookupUserIDByUsername(r.Context(), username)
		if userID == "" {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		invites, err := broker.BrokerListPendingInvites(r.Context(), userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(types.ListPendingInvitesResponse{Invites: invites})
	}))
	http.HandleFunc("/api/v0/rooms/invites/accept", withSignature(pool, withValidation(func(w http.ResponseWriter, r *http.Request, req types.AcceptInviteRequest) {
		username := r.Header.Get("X-Username")
		userID := broker.LookupUserIDByUsername(r.Context(), username)
		if userID == "" {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if err := broker.BrokerAcceptInvite(r.Context(), userID, req.RoomID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	http.HandleFunc("/api/v0/users/public-key", func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("username")
		if username == "" {
			http.Error(w, "missing username", http.StatusBadRequest)
			return
		}
		pub, err := broker.BrokerGetUserPublicKey(r.Context(), username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(types.GetUserPublicKeyResponse{PublicKey: pub})
	})
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
