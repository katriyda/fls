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
	"strconv"
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

func TestTusFullFlow(t *testing.T) {
	h, db, _ := setupTest(t)
	defer db.Close()

	content := []byte("hello world, this is a TUS upload test")
	contentLen := len(content)

	// Step 1: Create upload
	body := bytes.NewReader(nil)
	req := httptest.NewRequest("POST", "/api/upload/tus", body)
	req.Header.Set("Upload-Length", strconv.Itoa(contentLen))
	req.Header.Set("Upload-Metadata", "filename dGVzdC50eHQ=,filetype dGV4dC9wbGFpbg==")
	rec := httptest.NewRecorder()

	h.TusCreateUpload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}

	id := filepath.Base(location)
	if id == "" {
		t.Fatal("could not extract id from Location")
	}

	// Step 2: Head before any data
	req = httptest.NewRequest("HEAD", location, nil)
	rec = httptest.NewRecorder()
	h.TusHeadUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Upload-Offset") != "0" {
		t.Errorf("expected offset 0, got %s", rec.Header().Get("Upload-Offset"))
	}
	if rec.Header().Get("Upload-Length") != strconv.Itoa(contentLen) {
		t.Errorf("expected length %d, got %s", contentLen, rec.Header().Get("Upload-Length"))
	}

	// Step 3: Patch first chunk
	chunk1 := content[:6]
	req = httptest.NewRequest("PATCH", location, bytes.NewReader(chunk1))
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Upload-Offset", "0")
	rec = httptest.NewRecorder()

	h.TusPatchUpload(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Upload-Offset") != strconv.Itoa(len(chunk1)) {
		t.Errorf("expected offset %d, got %s", len(chunk1), rec.Header().Get("Upload-Offset"))
	}

	// Step 4: Head to check progress
	req = httptest.NewRequest("HEAD", location, nil)
	rec = httptest.NewRecorder()
	h.TusHeadUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Upload-Offset") != strconv.Itoa(len(chunk1)) {
		t.Errorf("expected offset %d, got %s", len(chunk1), rec.Header().Get("Upload-Offset"))
	}

	// Step 5: Patch remaining data
	req = httptest.NewRequest("PATCH", location, bytes.NewReader(content[6:]))
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Upload-Offset", strconv.Itoa(len(chunk1)))
	rec = httptest.NewRecorder()

	h.TusPatchUpload(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Upload-Offset") != strconv.Itoa(contentLen) {
		t.Errorf("expected offset %d, got %s", contentLen, rec.Header().Get("Upload-Offset"))
	}

	// Step 6: Verify file on disk
	var storagePath string
	var originalName string
	var mimeType string
	var fileSize int64
	db.QueryRow("SELECT storage_path, original_name, mime_type, size FROM files WHERE id = ?", id).Scan(&storagePath, &originalName, &mimeType, &fileSize)
	if storagePath == "" {
		t.Fatal("file record not found in database")
	}
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		t.Errorf("file not found on disk: %s", storagePath)
	}
	if originalName != "test.txt" {
		t.Errorf("expected test.txt, got %s", originalName)
	}
	if fileSize != int64(contentLen) {
		t.Errorf("expected size %d, got %d", contentLen, fileSize)
	}

	// Verify file content
	data, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch: expected %q, got %q", content, data)
	}

	// Step 7: Head should show completed
	req = httptest.NewRequest("HEAD", location, nil)
	rec = httptest.NewRecorder()
	h.TusHeadUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Upload-Complete") != "true" {
		t.Errorf("expected Upload-Complete header")
	}
}

func TestTusCreateEmptyUpload(t *testing.T) {
	h, db, _ := setupTest(t)
	defer db.Close()

	req := httptest.NewRequest("POST", "/api/upload/tus", nil)
	req.Header.Set("Upload-Length", "0")
	req.Header.Set("Upload-Metadata", "filename ZW1wdHkudHh0")
	rec := httptest.NewRecorder()

	h.TusCreateUpload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	id := filepath.Base(location)

	var count int
	db.QueryRow("SELECT COUNT(*) FROM files WHERE id = ?", id).Scan(&count)
	if count != 1 {
		t.Errorf("expected file record, got count %d", count)
	}

	var fileSize int64
	db.QueryRow("SELECT size FROM files WHERE id = ?", id).Scan(&fileSize)
	if fileSize != 0 {
		t.Errorf("expected size 0, got %d", fileSize)
	}
}

func TestTusCancelUpload(t *testing.T) {
	h, db, _ := setupTest(t)
	defer db.Close()

	req := httptest.NewRequest("POST", "/api/upload/tus", nil)
	req.Header.Set("Upload-Length", "100")
	req.Header.Set("Upload-Metadata", "filename Y2FuY2VsLnR4dA==")
	rec := httptest.NewRecorder()

	h.TusCreateUpload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	id := filepath.Base(location)

	// Cancel
	req = httptest.NewRequest("DELETE", location, nil)
	rec = httptest.NewRecorder()
	h.TusDeleteUpload(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// Verify cleanup - upload should no longer exist
	req = httptest.NewRequest("HEAD", location, nil)
	rec = httptest.NewRecorder()
	h.TusHeadUpload(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 after cancel, got %d", rec.Code)
	}

	// Verify no DB record
	var count int
	db.QueryRow("SELECT COUNT(*) FROM files WHERE id = ?", id).Scan(&count)
	if count != 0 {
		t.Errorf("expected no file record, got %d", count)
	}
}

func TestTusOffsetMismatch(t *testing.T) {
	h, db, _ := setupTest(t)
	defer db.Close()

	req := httptest.NewRequest("POST", "/api/upload/tus", nil)
	req.Header.Set("Upload-Length", "10")
	rec := httptest.NewRecorder()
	h.TusCreateUpload(rec, req)

	location := rec.Header().Get("Location")

	req = httptest.NewRequest("PATCH", location, bytes.NewReader([]byte("hello")))
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Upload-Offset", "5")
	rec = httptest.NewRecorder()
	h.TusPatchUpload(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 for offset mismatch, got %d", rec.Code)
	}
}

func TestTusMissingUploadLength(t *testing.T) {
	h, db, _ := setupTest(t)
	defer db.Close()

	req := httptest.NewRequest("POST", "/api/upload/tus", nil)
	rec := httptest.NewRecorder()
	h.TusCreateUpload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing Upload-Length, got %d", rec.Code)
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
