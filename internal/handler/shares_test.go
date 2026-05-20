package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fls/internal/database"
	"fls/internal/service"

	"github.com/go-chi/chi/v5"
)

func openTestDB(t *testing.T) *database.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := database.New(path)
	if err != nil {
		t.Fatalf("New(%q) error = %v", path, err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func setupShareHandler(t *testing.T) (*database.DB, *ShareHandler, *service.ShareService) {
	t.Helper()
	db := openTestDB(t)
	shareSvc := service.NewShareService(db.DB, nil)
	h := NewShareHandler(db.DB, shareSvc, nil)
	return db, h, shareSvc
}

func setupChiRouter(h *ShareHandler) chi.Router {
	r := chi.NewRouter()
	r.Get("/admin/shares", h.ListShares)
	r.Get("/admin/shares/new", h.NewShareForm)
	r.Post("/admin/shares", h.CreateShare)
	r.Get("/admin/shares/{id}", h.GetShare)
	r.Delete("/admin/shares/{id}", h.DeleteShare)
	r.Get("/admin/shares/{id}/qrcode", h.QRCode)
	return r
}

func TestListShares_Empty(t *testing.T) {
	_, h, _ := setupShareHandler(t)
	r := setupChiRouter(h)

	req := httptest.NewRequest("GET", "/admin/shares", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestListShares_WithData(t *testing.T) {
	db, h, svc := setupShareHandler(t)

	fileID := "test-file-id"
	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		fileID, "test.txt", "test.txt", 1024, "/tmp/test.txt",
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	_, err = svc.CreateTextShare("hello world", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	r := setupChiRouter(h)
	req := httptest.NewRequest("GET", "/admin/shares", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestNewShareForm(t *testing.T) {
	_, h, _ := setupShareHandler(t)
	r := setupChiRouter(h)

	req := httptest.NewRequest("GET", "/admin/shares/new", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestNewShareForm_WithFiles(t *testing.T) {
	db, h, _ := setupShareHandler(t)

	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		"file1", "doc.txt", "document.txt", 512, "/tmp/doc.txt",
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	r := setupChiRouter(h)
	req := httptest.NewRequest("GET", "/admin/shares/new", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCreateShare_Text(t *testing.T) {
	_, h, _ := setupShareHandler(t)
	r := setupChiRouter(h)

	body := "content_type=text&text_content=hello+world&expires_in=24h&max_downloads=5"
	req := httptest.NewRequest("POST", "/admin/shares", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("HX-Redirect") != "/admin/shares" {
		t.Errorf("expected HX-Redirect header, got %q", rec.Header().Get("HX-Redirect"))
	}
}

func TestCreateShare_File(t *testing.T) {
	db, h, _ := setupShareHandler(t)

	fileID := "test-file-id"
	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		fileID, "test.txt", "test.txt", 1024, "/tmp/test.txt",
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	r := setupChiRouter(h)
	body := "content_type=file&file_id=" + fileID + "&expires_in=7d"
	req := httptest.NewRequest("POST", "/admin/shares", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCreateShare_WithPassword(t *testing.T) {
	_, h, _ := setupShareHandler(t)
	r := setupChiRouter(h)

	body := "content_type=text&text_content=secret&password=mypassword&expires_in=1h"
	req := httptest.NewRequest("POST", "/admin/shares", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCreateShare_MissingText(t *testing.T) {
	_, h, _ := setupShareHandler(t)
	r := setupChiRouter(h)

	body := "content_type=text&text_content="
	req := httptest.NewRequest("POST", "/admin/shares", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing text, got %d", rec.Code)
	}
}

func TestCreateShare_MissingFileID(t *testing.T) {
	_, h, _ := setupShareHandler(t)
	r := setupChiRouter(h)

	body := "content_type=file"
	req := httptest.NewRequest("POST", "/admin/shares", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file_id, got %d", rec.Code)
	}
}

func TestCreateShare_InvalidContentType(t *testing.T) {
	_, h, _ := setupShareHandler(t)
	r := setupChiRouter(h)

	body := "content_type=invalid"
	req := httptest.NewRequest("POST", "/admin/shares", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid content_type, got %d", rec.Code)
	}
}

func TestGetShare_NotFound(t *testing.T) {
	_, h, _ := setupShareHandler(t)
	r := setupChiRouter(h)

	req := httptest.NewRequest("GET", "/admin/shares/nonexistent-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGetShare_Text(t *testing.T) {
	_, h, svc := setupShareHandler(t)

	share, err := svc.CreateTextShare("test content", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	r := setupChiRouter(h)
	req := httptest.NewRequest("GET", "/admin/shares/"+share.ID, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestGetShare_File(t *testing.T) {
	db, h, svc := setupShareHandler(t)

	fileID := "test-file-id"
	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		fileID, "test.txt", "test.txt", 1024, "/tmp/test.txt",
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	share, err := svc.CreateFileShare(fileID, "", nil, 3)
	if err != nil {
		t.Fatalf("CreateFileShare: %v", err)
	}

	r := setupChiRouter(h)
	req := httptest.NewRequest("GET", "/admin/shares/"+share.ID, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestDeleteShare(t *testing.T) {
	_, h, svc := setupShareHandler(t)

	share, err := svc.CreateTextShare("delete me", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	r := setupChiRouter(h)
	req := httptest.NewRequest("DELETE", "/admin/shares/"+share.ID, nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("HX-Redirect") != "/admin/shares" {
		t.Errorf("expected HX-Redirect header, got %q", rec.Header().Get("HX-Redirect"))
	}
}

func TestDeleteShare_NotFound(t *testing.T) {
	_, h, _ := setupShareHandler(t)
	r := setupChiRouter(h)

	req := httptest.NewRequest("DELETE", "/admin/shares/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestQRCode_NotFound(t *testing.T) {
	_, h, _ := setupShareHandler(t)
	r := setupChiRouter(h)

	req := httptest.NewRequest("GET", "/admin/shares/nonexistent/qrcode", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestQRCode_Success(t *testing.T) {
	_, h, svc := setupShareHandler(t)

	share, err := svc.CreateTextShare("qr test", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	r := setupChiRouter(h)
	req := httptest.NewRequest("GET", "/admin/shares/"+share.ID+"/qrcode", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected image/png, got %q", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty QR code image")
	}
}

func TestParseExpiresIn(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"never", false},
		{"1h", true},
		{"24h", true},
		{"7d", true},
		{"30d", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		result := parseExpiresIn(tt.input)
		if tt.want && result == nil {
			t.Errorf("parseExpiresIn(%q) = nil, want non-nil", tt.input)
		}
		if !tt.want && result != nil {
			t.Errorf("parseExpiresIn(%q) = non-nil, want nil", tt.input)
		}
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
