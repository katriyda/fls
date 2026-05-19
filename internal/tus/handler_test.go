package tus

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTest(t *testing.T) (*Handler, *sql.DB, string) {
	t.Helper()

	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY, filename TEXT NOT NULL,
			original_name TEXT NOT NULL, size INTEGER NOT NULL,
			mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
			storage_path TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	return New(db, dataDir), db, dataDir
}

func TestSimpleUpload(t *testing.T) {
	h, db, _ := setupTest(t)
	defer db.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("hello world"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/simple", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.SimpleUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if result["original_name"] != "test.txt" {
		t.Errorf("expected test.txt, got %v", result["original_name"])
	}
	if result["size"] != float64(11) {
		t.Errorf("expected 11, got %v", result["size"])
	}

	var storagePath string
	if sp, ok := result["storage_path"]; ok {
		storagePath, _ = sp.(string)
	}
	if storagePath != "" {
		if _, err := os.Stat(storagePath); os.IsNotExist(err) {
			t.Error("file not found on disk")
		}
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM files WHERE id = ?", result["id"]).Scan(&count)
	if count != 1 {
		t.Error("file record not found in database")
	}
}

func TestSimpleUploadMissingFile(t *testing.T) {
	h, db, _ := setupTest(t)
	defer db.Close()

	req := httptest.NewRequest("POST", "/api/upload/simple", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	rec := httptest.NewRecorder()

	h.SimpleUpload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestSimpleUploadEmpty(t *testing.T) {
	h, db, _ := setupTest(t)
	defer db.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "empty.txt")
	part.Write([]byte{})
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/simple", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.SimpleUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty file, got %d: %s", rec.Code, rec.Body.String())
	}
}
