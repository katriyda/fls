package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRenderTemplate(t *testing.T) {
	_ = templates
}

func TestStaticHandler(t *testing.T) {
	handler := StaticHandler()
	req := httptest.NewRequest("GET", "/custom.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
