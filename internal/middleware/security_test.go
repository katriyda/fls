package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"fls/internal/config"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := SecurityHeadersMiddleware(handler)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	headers := []struct {
		key, val string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "no-referrer"},
		{"X-XSS-Protection", "0"},
	}

	for _, h := range headers {
		if got := rec.Header().Get(h.key); got != h.val {
			t.Errorf("header %s = %q, want %q", h.key, got, h.val)
		}
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header is missing")
	}
}

func TestDynamicRateLimitMiddleware(t *testing.T) {
	cfg := config.Defaults()
	cfg.RateLimitPerMinute = 10

	// Test non-login rate limit
	mw := DynamicRateLimitMiddleware(cfg, false)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := mw(handler)

	// Make 10 requests, which should succeed
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d failed with status %d", i+1, rec.Code)
		}
	}

	// 11th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rec.Code)
	}

	var res map[string]string
	json.NewDecoder(rec.Body).Decode(&res)
	if res["error"] != "rate limit exceeded" {
		t.Errorf("expected rate limit exceeded error, got %v", res["error"])
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		baseDir    string
		targetPath string
		expectErr  bool
	}{
		{"/data/uploads", "/data/uploads/file.txt", false},
		{"/data/uploads", "/data/uploads/sub/file.txt", false},
		{"/data/uploads", "/data/uploads/../file.txt", true},
		{"/data/uploads", "/etc/passwd", true},
	}

	for _, tc := range tests {
		// Normalise path separators for current platform (Windows/Unix)
		base := filepath.FromSlash(tc.baseDir)
		target := filepath.FromSlash(tc.targetPath)
		err := ValidatePath(base, target)
		if tc.expectErr && err == nil {
			t.Errorf("expected error for ValidatePath(%q, %q), got nil", base, target)
		}
		if !tc.expectErr && err != nil {
			t.Errorf("unexpected error for ValidatePath(%q, %q): %v", base, target, err)
		}
	}
}
