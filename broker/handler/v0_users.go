package handler

import (
	"encoding/json"
	"net/http"

	apitypes "github.com/srschreiber/nito/api_types"
)

func (h *Handler) getUserPublicKey(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "missing username", http.StatusBadRequest)
		return
	}
	pub, err := h.broker.BrokerGetUserPublicKey(r.Context(), username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apitypes.GetUserPublicKeyResponse{PublicKey: pub})
}
