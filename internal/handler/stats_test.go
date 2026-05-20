package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fls/internal/service"

	"github.com/go-chi/chi/v5"
)

func TestStatsHandler_GlobalStats(t *testing.T) {
	_, sqldb := setupTestDB(t)
	statsSvc := service.NewStatsService(sqldb)
	h := NewStatsHandler(statsSvc)

	r := chi.NewRouter()
	r.Get("/admin/api/stats", h.GetStats)

	req := httptest.NewRequest("GET", "/admin/api/stats", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var result service.GlobalStats
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0", result.TotalFiles)
	}
}

func TestStatsHandler_ShareStats(t *testing.T) {
	_, sqldb := setupTestDB(t)
	statsSvc := service.NewStatsService(sqldb)
	shareSvc := service.NewShareService(sqldb, nil)
	h := NewStatsHandler(statsSvc)

	share, err := shareSvc.CreateTextShare("test", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	err = statsSvc.RecordDownload(share.ID, "1.2.3.4", "agent")
	if err != nil {
		t.Fatalf("RecordDownload: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/admin/api/stats", h.GetStats)

	req := httptest.NewRequest("GET", "/admin/api/stats?share_id="+share.ID, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var result service.ShareStats
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.TotalDownloads != 1 {
		t.Errorf("TotalDownloads = %d, want 1", result.TotalDownloads)
	}
	if result.UniqueIPs != 1 {
		t.Errorf("UniqueIPs = %d, want 1", result.UniqueIPs)
	}
}

func TestStatsHandler_JSONContentType(t *testing.T) {
	_, sqldb := setupTestDB(t)
	statsSvc := service.NewStatsService(sqldb)
	h := NewStatsHandler(statsSvc)

	r := chi.NewRouter()
	r.Get("/admin/api/stats", h.GetStats)

	req := httptest.NewRequest("GET", "/admin/api/stats", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
