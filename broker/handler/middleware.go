package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/srschreiber/nito/broker/auth"
	"github.com/srschreiber/nito/broker/database"
)

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
