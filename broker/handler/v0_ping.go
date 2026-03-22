package handler

import (
	"encoding/json"
	"net/http"

	apitypes "github.com/srschreiber/nito/api_types"
)

func (h *Handler) ping(w http.ResponseWriter, _ *http.Request, req apitypes.PingRequest) {
	resp := apitypes.PingResponse{Message: req.Message}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
