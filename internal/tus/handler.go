package tus

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type uploadInfo struct {
	mu          sync.Mutex
	id          string
	size        int64
	offset      int64
	metadata    map[string]string
	storagePath string
	isFinished  bool
	tempDir     string
}

type Handler struct {
	db      *sql.DB
	dataDir string
	uploads sync.Map
}

func New(db *sql.DB, dataDir string) *Handler {
	return &Handler{db: db, dataDir: dataDir}
}

func (h *Handler) Mount() http.Handler {
	r := chi.NewRouter()
	r.Post("/tus", h.TusCreateUpload)
	r.Patch("/tus/{id}", h.TusPatchUpload)
	r.Head("/tus/{id}", h.TusHeadUpload)
	r.Delete("/tus/{id}", h.TusDeleteUpload)
	r.Post("/simple", h.SimpleUpload)
	return r
}

func (h *Handler) SimpleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<30)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	id := uuid.New().String()
	now := time.Now()
	dateDir := now.Format("2006-01")
	storageDir := filepath.Join(h.dataDir, "uploads", dateDir)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		http.Error(w, "failed to create storage: "+err.Error(), http.StatusInternalServerError)
		return
	}

	storedName := id + "_" + header.Filename
	storagePath := filepath.Join(storageDir, storedName)

	dst, err := os.Create(storagePath)
	if err != nil {
		http.Error(w, "failed to create file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(storagePath)
		http.Error(w, "failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	buf := make([]byte, 512)
	dst.Seek(0, 0)
	n, _ := dst.Read(buf)
	mimeType := http.DetectContentType(buf[:n])

	_, err = h.db.Exec(
		`INSERT INTO files (id, filename, original_name, size, mime_type, storage_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, storedName, header.Filename, written, mimeType, storagePath,
	)
	if err != nil {
		os.Remove(storagePath)
		http.Error(w, "failed to save record: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("file uploaded", "id", id, "name", header.Filename, "size", written)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":            id,
		"filename":      storedName,
		"original_name": header.Filename,
		"size":          written,
		"mime_type":     mimeType,
	})
}
