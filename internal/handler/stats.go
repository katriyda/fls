package handler

import (
	"encoding/json"
	"net/http"

	"fls/internal/service"
)

type StatsHandler struct {
	statsSvc *service.StatsService
}

func NewStatsHandler(statsSvc *service.StatsService) *StatsHandler {
	return &StatsHandler{statsSvc: statsSvc}
}

func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	shareID := r.URL.Query().Get("share_id")

	if shareID != "" {
		stats, err := h.statsSvc.GetShareStats(shareID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
		return
	}

	stats, err := h.statsSvc.GetGlobalStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
