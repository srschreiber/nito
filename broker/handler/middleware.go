package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/srschreiber/nito/broker/auth"
	"github.com/srschreiber/nito/broker/database"
)

// VerifyJWTToken validates a raw JWT token string and returns the username claim.
func VerifyJWTToken(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims")
	}
	username, ok := claims["username"].(string)
	if !ok || username == "" {
		return "", errors.New("missing username claim")
	}
	return username, nil
}

// withSignature validates the X-Username, X-Signature, and Authorization: Bearer headers.
// The expected signed payload is "username:path" (e.g. "alice:/api/v0/rooms").
func withSignature(db *pgxpool.Pool, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify JWT session token.
		authHeader := r.Header.Get("Authorization")
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}
		jwtUsername, err := VerifyJWTToken(authHeader[len(bearerPrefix):])
		if err != nil {
			http.Error(w, "invalid JWT: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Verify request signature.
		username := r.Header.Get("X-Username")
		sigB64 := r.Header.Get("X-Signature")
		if username == "" || sigB64 == "" {
			http.Error(w, "missing signature headers", http.StatusUnauthorized)
			return
		}
		if username != jwtUsername {
			http.Error(w, "username mismatch between JWT and signature header", http.StatusUnauthorized)
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
