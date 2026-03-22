package handler

import (
	"context"
	"net/http"
)

func (h *Handler) ws(w http.ResponseWriter, r *http.Request) {
	h.broker.WsConnect(context.Background(), w, r)
}
