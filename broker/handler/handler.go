package handler

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	brokerws "github.com/srschreiber/nito/broker/websocket"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	broker *brokerws.Broker
	pool   *pgxpool.Pool
}

// New creates a Handler with the given broker and pool.
func New(broker *brokerws.Broker, pool *pgxpool.Pool) *Handler {
	return &Handler{broker: broker, pool: pool}
}

// Register registers all API routes on the default mux.
func (h *Handler) Register() {
	InitLoginHandler()
	h.broker.JWTVerifier = VerifyJWTToken

	http.HandleFunc("/api/v0/ping", withValidation(h.ping))
	http.HandleFunc("/api/v0/register", withValidation(h.register))
	http.HandleFunc("/api/v0/login/challenge", withValidation(h.challenge))
	http.HandleFunc("/api/v0/login", withValidation(h.login))
	http.HandleFunc("/api/v0/rooms", withSignature(h.pool, withValidation(h.createRoom)))
	http.HandleFunc("/api/v0/rooms/list", withSignature(h.pool, h.listRooms))
	http.HandleFunc("/api/v0/rooms/invite", withSignature(h.pool, withValidation(h.inviteUser)))
	http.HandleFunc("/api/v0/rooms/members", withSignature(h.pool, h.listRoomMembers))
	http.HandleFunc("/api/v0/rooms/key", withSignature(h.pool, h.getRoomKey))
	http.HandleFunc("/api/v0/rooms/info", withSignature(h.pool, h.getRoomInfo))
	http.HandleFunc("/api/v0/rooms/invites", withSignature(h.pool, h.listPendingInvites))
	http.HandleFunc("/api/v0/rooms/invites/accept", withSignature(h.pool, withValidation(h.acceptInvite)))
	http.HandleFunc("/api/v0/users/public-key", h.getUserPublicKey)
	http.HandleFunc("/ws", h.ws)
}
