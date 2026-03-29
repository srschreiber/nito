// Copyright 2026 Sam Schreiber
//
// This file is part of nito.
//
// nito is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// nito is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with nito. If not, see <https://www.gnu.org/licenses/>.

package handler

import (
	"context"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	brokercore "github.com/srschreiber/nito/broker/core"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	broker *brokercore.Broker
	pool   *pgxpool.Pool
	ctx    context.Context
}

// New creates a Handler with the given broker and pool.
func New(broker *brokercore.Broker, pool *pgxpool.Pool) *Handler {
	return &Handler{broker: broker, pool: pool, ctx: context.Background()}
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
	http.HandleFunc("/api/v0/rooms/messages", withSignature(h.pool, withValidation(h.GetRoomMessages)))
	http.HandleFunc("/ws", h.ws)
}
