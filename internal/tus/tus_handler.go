package tus

import (
	"net/http"
)

func (h *Handler) TusHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "TUS upload not yet implemented", http.StatusNotImplemented)
}
