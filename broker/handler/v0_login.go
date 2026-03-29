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
	"encoding/json"
	"os"

	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/srschreiber/nito/broker/auth"
	"github.com/srschreiber/nito/broker/database"

	"github.com/ReneKroon/ttlcache"
	apitypes "github.com/srschreiber/nito/shared/api_types"
)

var loginChallenges *ttlcache.Cache = nil
var jwtSecret string = ""

func InitLoginHandler() {
	loginChallenges = ttlcache.NewCache()
	loginChallenges.SetTTL(5 * time.Minute)
	jwtSecret = os.Getenv("JWT_SECRET") // symmetric key for signing JWTs
	if jwtSecret == "" {
		// randomize it, breaking existing sessions
		jwtSecret = uuid.NewString()
	}
}

func (h *Handler) challenge(w http.ResponseWriter, _ *http.Request, req apitypes.LoginChallengeRequest) {
	challenge := uuid.NewString()
	loginChallenges.Set(challenge, true)
	resp := apitypes.LoginChallengeResponse{Challenge: challenge}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) login(w http.ResponseWriter, _ *http.Request, req apitypes.LoginRequest) {
	// first, simply check the challenge is valid and evict it
	if _, exists := loginChallenges.Get(req.Challenge); !exists {
		http.Error(w, "invalid or expired challenge", http.StatusUnauthorized)
		return
	}
	loginChallenges.Remove(req.Challenge)
	valid, err := database.ValidateUserPassword(context.Background(), h.broker.DB, req.Username, req.Password)
	if err != nil {
		http.Error(w, "could not verify user", http.StatusUnauthorized)
		return
	}
	if !valid {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	pubKeyPEM, err := database.GetUserPublicKeyByUsername(context.Background(), h.broker.DB, req.Username)
	if err != nil || pubKeyPEM == nil {
		http.Error(w, "user has no public key", http.StatusUnauthorized)
		return
	}
	unsignedMessage := auth.FormatMessageForLoginSigning(req.Username, req.Challenge)
	if err := auth.VerifySignature(*pubKeyPEM, unsignedMessage, req.Signature); err != nil {
		http.Error(w, "invalid signature: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// create JWT, now that user has cleared all checks
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": req.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}
	resp := apitypes.LoginResponse{Token: tokenString}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}

}

func (h *Handler) logout(w http.ResponseWriter, _ *http.Request) {

}

func (h *Handler) refreshToken(w http.ResponseWriter, _ *http.Request) {

}
