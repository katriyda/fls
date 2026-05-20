package handler

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"fls/internal/database"

	"github.com/go-chi/chi/v5"
)

func setupTestDB(t *testing.T) (*database.DB, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("database.New() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("db.Migrate() error = %v", err)
	}
	return db, db.DB
}

func seedTestFile(db *sql.DB, id, filename, originalName string, size int64, mimeType, storagePath string) {
	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, mime_type, storage_path) VALUES (?, ?, ?, ?, ?, ?)`,
		id, filename, originalName, size, mimeType, storagePath,
	)
	if err != nil {
		panic("seed file: " + err.Error())
	}
}

func TestFileHandler_ListFiles(t *testing.T) {
	_, sqldb := setupTestDB(t)
	seedTestFile(sqldb, "f1", "report.pdf", "annual-report.pdf", 204800, "application/pdf", "/tmp/report.pdf")
	seedTestFile(sqldb, "f2", "photo.jpg", "vacation-photo.jpg", 1048576, "image/jpeg", "/tmp/photo.jpg")

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Get("/admin/files", fh.ListFiles)

	req := httptest.NewRequest("GET", "/admin/files", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListFiles status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "annual-report.pdf") {
		t.Errorf("ListFiles body should contain annual-report.pdf")
	}
	if !strings.Contains(rec.Body.String(), "vacation-photo.jpg") {
		t.Errorf("ListFiles body should contain vacation-photo.jpg")
	}
}

func TestFileHandler_ListFiles_Search(t *testing.T) {
	_, sqldb := setupTestDB(t)
	seedTestFile(sqldb, "f1", "report.pdf", "annual-report.pdf", 204800, "application/pdf", "/tmp/report.pdf")
	seedTestFile(sqldb, "f2", "photo.jpg", "vacation-photo.jpg", 1048576, "image/jpeg", "/tmp/photo.jpg")

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Get("/admin/files", fh.ListFiles)

	req := httptest.NewRequest("GET", "/admin/files?search=report", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListFiles search status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "annual-report.pdf") {
		t.Errorf("ListFiles search body should contain annual-report.pdf")
	}
	if strings.Contains(rec.Body.String(), "vacation-photo.jpg") {
		t.Errorf("ListFiles search body should NOT contain vacation-photo.jpg")
	}
}

func TestFileHandler_GetFile(t *testing.T) {
	_, sqldb := setupTestDB(t)
	seedTestFile(sqldb, "f1", "report.pdf", "annual-report.pdf", 204800, "application/pdf", "/tmp/report.pdf")

	// Seed a share with NULL password_hash and NULL text_content to verify Scan safety.
	_, err := sqldb.Exec(
		`INSERT INTO shares (id, file_id, token, password_hash, expires_at, max_downloads, download_count, content_type, text_content)
		 VALUES (?, ?, ?, NULL, NULL, 0, 0, 'file', NULL)`,
		"s1", "f1", "test-token",
	)
	if err != nil {
		t.Fatalf("failed to seed test share: %v", err)
	}

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Get("/admin/files/{id}", fh.GetFile)

	req := httptest.NewRequest("GET", "/admin/files/f1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GetFile status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "annual-report.pdf") {
		t.Errorf("GetFile body should contain annual-report.pdf")
	}
	if !strings.Contains(rec.Body.String(), "application/pdf") {
		t.Errorf("GetFile body should contain MIME type")
	}
}

func TestFileHandler_GetFile_NotFound(t *testing.T) {
	_, sqldb := setupTestDB(t)

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Get("/admin/files/{id}", fh.GetFile)

	req := httptest.NewRequest("GET", "/admin/files/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GetFile nonexistent status = %d, want 404", rec.Code)
	}
}

func TestFileHandler_DeleteFile(t *testing.T) {
	_, sqldb := setupTestDB(t)
	seedTestFile(sqldb, "f1", "report.pdf", "annual-report.pdf", 204800, "application/pdf", "/tmp/report.pdf")

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Delete("/admin/files/{id}", fh.DeleteFile)

	req := httptest.NewRequest("DELETE", "/admin/files/f1", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("DeleteFile status = %d, want 200", rec.Code)
	}

	hxRedirect := rec.Header().Get("HX-Redirect")
	if hxRedirect != "/admin/files" {
		t.Errorf("DeleteFile HX-Redirect = %q, want /admin/files", hxRedirect)
	}

	// Verify file is deleted from DB
	var count int
	sqldb.QueryRow("SELECT COUNT(*) FROM files WHERE id = ?", "f1").Scan(&count)
	if count != 0 {
		t.Errorf("file should be deleted from DB")
	}
}

func TestFileHandler_DeleteFile_NotFound(t *testing.T) {
	_, sqldb := setupTestDB(t)

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Delete("/admin/files/{id}", fh.DeleteFile)

	req := httptest.NewRequest("DELETE", "/admin/files/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("DeleteFile nonexistent status = %d, want 404", rec.Code)
	}
}

func TestFileHandler_EditFile(t *testing.T) {
	_, sqldb := setupTestDB(t)
	seedTestFile(sqldb, "f1", "report.pdf", "annual-report.pdf", 204800, "application/pdf", "/tmp/report.pdf")

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Get("/admin/files/{id}/edit", fh.EditFile)

	req := httptest.NewRequest("GET", "/admin/files/f1/edit", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("EditFile status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "保存") {
		t.Errorf("EditFile body should contain save button")
	}
}

func TestFileHandler_UpdateFile(t *testing.T) {
	_, sqldb := setupTestDB(t)
	seedTestFile(sqldb, "f1", "report.pdf", "annual-report.pdf", 204800, "application/pdf", "/tmp/report.pdf")

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Put("/admin/files/{id}", fh.UpdateFile)

	body := "original_name=updated-name.pdf"
	req := httptest.NewRequest("PUT", "/admin/files/f1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("UpdateFile status = %d, want 303", rec.Code)
	}

	// Verify update
	var name string
	sqldb.QueryRow("SELECT original_name FROM files WHERE id = ?", "f1").Scan(&name)
	if name != "updated-name.pdf" {
		t.Errorf("original_name after update = %q, want %q", name, "updated-name.pdf")
	}
}

func TestFileHandler_UpdateFile_NotFound(t *testing.T) {
	_, sqldb := setupTestDB(t)

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Put("/admin/files/{id}", fh.UpdateFile)

	body := "original_name=test.pdf"
	req := httptest.NewRequest("PUT", "/admin/files/nonexistent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("UpdateFile nonexistent status = %d, want 404", rec.Code)
	}
}

func TestFileHandler_UpdateFile_EmptyName(t *testing.T) {
	_, sqldb := setupTestDB(t)
	seedTestFile(sqldb, "f1", "report.pdf", "annual-report.pdf", 204800, "application/pdf", "/tmp/report.pdf")

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Put("/admin/files/{id}", fh.UpdateFile)

	body := "original_name="
	req := httptest.NewRequest("PUT", "/admin/files/f1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("UpdateFile empty name status = %d, want 400", rec.Code)
	}
}

func TestFileHandler_ListFiles_Empty(t *testing.T) {
	_, sqldb := setupTestDB(t)

	fh := NewFileHandler(sqldb)
	r := chi.NewRouter()
	r.Get("/admin/files", fh.ListFiles)

	req := httptest.NewRequest("GET", "/admin/files", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ListFiles empty status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "暂无文件") {
		t.Errorf("ListFiles empty body should contain '暂无文件'")
	}
}
