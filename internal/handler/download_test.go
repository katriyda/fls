package handler

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fls/internal/database"
	"fls/internal/service"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
)

func setupDownloadTest(t *testing.T) (*database.DB, *DownloadHandler, *service.ShareService) {
	t.Helper()
	_, sqldb := setupTestDB(t)
	shareSvc := service.NewShareService(sqldb, nil)
	statsSvc := service.NewStatsService(sqldb)
	sm := scs.New()
	h := NewDownloadHandler(sqldb, shareSvc, statsSvc, sm)
	return nil, h, shareSvc
}

func setupDownloadRouter(h *DownloadHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/s/{token}", h.ServeShare)
	r.Post("/s/{token}", h.VerifySharePassword)
	r.Get("/s/{token}/raw", h.RawContent)
	r.Get("/s/{token}/download", h.DownloadFile)
	if h.sm != nil {
		return h.sm.LoadAndSave(r)
	}
	return r
}

func setupDownloadWithFile(t *testing.T, sqldb *sql.DB) (string, string) {
	t.Helper()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-file.txt")
	if err := os.WriteFile(filePath, []byte("hello world"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	fileID := "download-test-file"
	_, err := sqldb.Exec(
		`INSERT INTO files (id, filename, original_name, size, mime_type, storage_path) VALUES (?, ?, ?, ?, ?, ?)`,
		fileID, "test-file.txt", "test-file.txt", 11, "text/plain", filePath,
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}
	return fileID, dir
}

func TestDownload_TextShare_RendersText(t *testing.T) {
	_, h, svc := setupDownloadTest(t)
	share, err := svc.CreateTextShare("hello world content", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	r := setupDownloadRouter(h)
	req := httptest.NewRequest("GET", "/s/"+share.Token, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ServeShare status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hello world content") {
		t.Errorf("body should contain text content")
	}
}

func TestDownload_ExpiredShare_ShowsExpired(t *testing.T) {
	_, h, svc := setupDownloadTest(t)
	_, err := svc.CreateTextShare("expired content", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	// Manually set expires_at to the past
	_, err = h.db.Exec("UPDATE shares SET expires_at = datetime('now', '-1 day') WHERE text_content = 'expired content'")
	if err != nil {
		t.Fatalf("update share: %v", err)
	}

	// Get the token
	var token string
	err = h.db.QueryRow("SELECT token FROM shares WHERE text_content = 'expired content'").Scan(&token)
	if err != nil {
		t.Fatalf("get token: %v", err)
	}

	r := setupDownloadRouter(h)
	req := httptest.NewRequest("GET", "/s/"+token, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ServeShare expired status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "链接已失效/已过期") {
		t.Errorf("body should contain expired message")
	}
}

func TestDownload_PasswordPage_ShowsForm(t *testing.T) {
	_, h, svc := setupDownloadTest(t)
	pwHash, err := service.HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	share, err := svc.CreateTextShare("protected text", pwHash, nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	r := setupDownloadRouter(h)
	req := httptest.NewRequest("GET", "/s/"+share.Token, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ServeShare password status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "需要密码") {
		t.Errorf("body should contain password prompt")
	}
}

func TestDownload_CorrectPassword_VerifiesAndShowsContent(t *testing.T) {
	_, h, svc := setupDownloadTest(t)
	pwHash, err := service.HashPassword("correctpw")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	share, err := svc.CreateTextShare("protected content", pwHash, nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	r := setupDownloadRouter(h)

	body := "password=correctpw"
	req := httptest.NewRequest("POST", "/s/"+share.Token, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("VerifySharePassword status = %d, want 303", rec.Code)
	}

	redirectURL := rec.Header().Get("Location")
	if redirectURL != "/s/"+share.Token {
		t.Errorf("redirect location = %q, want /s/%s", redirectURL, share.Token)
	}

	// We must pass the session cookie from the response to the next request to preserve session state!
	req2 := httptest.NewRequest("GET", "/s/"+share.Token, nil)
	for _, cookie := range rec.Result().Cookies() {
		req2.AddCookie(cookie)
	}
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("ServeShare after password status = %d, want 200", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "protected content") {
		t.Errorf("body should contain text content after password verification")
	}
}

func TestDownload_WrongPassword_ShowsError(t *testing.T) {
	_, h, svc := setupDownloadTest(t)
	pwHash, err := service.HashPassword("correctpw")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	share, err := svc.CreateTextShare("secret", pwHash, nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	r := setupDownloadRouter(h)

	body := "password=wrongpw"
	req := httptest.NewRequest("POST", "/s/"+share.Token, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("VerifySharePassword wrong status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "密码错误") {
		t.Errorf("body should contain error message")
	}
}

func TestDownload_FileEndpoint_StreamsFile(t *testing.T) {
	db, sqldb := setupTestDB(t)
	shareSvc := service.NewShareService(sqldb, nil)
	statsSvc := service.NewStatsService(sqldb)
	sm := scs.New()
	h := NewDownloadHandler(sqldb, shareSvc, statsSvc, sm)

	fileID, _ := setupDownloadWithFile(t, sqldb)

	share, err := shareSvc.CreateFileShare(fileID, "", nil, 0)
	if err != nil {
		t.Fatalf("CreateFileShare: %v", err)
	}

	_ = db
	r := setupDownloadRouter(h)
	req := httptest.NewRequest("GET", "/s/"+share.Token+"/download", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("DownloadFile status = %d, want 200", rec.Code)
	}

	if rec.Body.String() != "hello world" {
		t.Errorf("download body = %q, want %q", rec.Body.String(), "hello world")
	}

	disp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "attachment") {
		t.Errorf("Content-Disposition should contain attachment, got %q", disp)
	}
}

func TestDownload_IncrementsCounter(t *testing.T) {
	_, sqldb := setupTestDB(t)
	shareSvc := service.NewShareService(sqldb, nil)
	statsSvc := service.NewStatsService(sqldb)
	sm := scs.New()
	h := NewDownloadHandler(sqldb, shareSvc, statsSvc, sm)

	fileID, _ := setupDownloadWithFile(t, sqldb)

	share, err := shareSvc.CreateFileShare(fileID, "", nil, 0)
	if err != nil {
		t.Fatalf("CreateFileShare: %v", err)
	}

	r := setupDownloadRouter(h)
	req := httptest.NewRequest("GET", "/s/"+share.Token+"/download", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("DownloadFile status = %d, want 200", rec.Code)
	}

	var downloadCount int
	err = sqldb.QueryRow("SELECT download_count FROM shares WHERE id = ?", share.ID).Scan(&downloadCount)
	if err != nil {
		t.Fatalf("query download_count: %v", err)
	}
	if downloadCount != 1 {
		t.Errorf("download_count = %d, want 1", downloadCount)
	}

	var logCount int
	err = sqldb.QueryRow("SELECT COUNT(*) FROM download_logs WHERE share_id = ?", share.ID).Scan(&logCount)
	if err != nil {
		t.Fatalf("query download_logs: %v", err)
	}
	if logCount != 1 {
		t.Errorf("download_logs count = %d, want 1", logCount)
	}
}
