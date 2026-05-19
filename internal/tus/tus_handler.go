package tus

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func parseUploadMetadata(header string) map[string]string {
	m := make(map[string]string)
	for _, pair := range strings.Split(header, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, " ", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		if len(parts) == 2 {
			value := strings.TrimSpace(parts[1])
			if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
				m[key] = string(decoded)
			} else {
				m[key] = value
			}
		} else {
			m[key] = ""
		}
	}
	return m
}

func encodeUploadMetadata(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}
	parts := make([]string, 0, len(metadata))
	for k, v := range metadata {
		encoded := base64.StdEncoding.EncodeToString([]byte(v))
		parts = append(parts, fmt.Sprintf("%s %s", k, encoded))
	}
	return strings.Join(parts, ",")
}

func (h *Handler) finalizeUpload(info *uploadInfo) error {
	originalName := info.metadata["filename"]
	if originalName == "" {
		originalName = info.id
	}

	finalPath := filepath.Join(info.tempDir, originalName)

	mimeType := "application/octet-stream"
	if sniff, err := os.ReadFile(info.storagePath); err == nil && len(sniff) > 0 {
		headerBytes := sniff
		if len(headerBytes) > 512 {
			headerBytes = headerBytes[:512]
		}
		mimeType = http.DetectContentType(headerBytes)
	}

	if info.storagePath != finalPath {
		if err := os.Rename(info.storagePath, finalPath); err != nil {
			src, err2 := os.Open(info.storagePath)
			if err2 != nil {
				return fmt.Errorf("open temp file: %w", err2)
			}
			defer src.Close()
			dst, err2 := os.Create(finalPath)
			if err2 != nil {
				return fmt.Errorf("create final file: %w", err2)
			}
			defer dst.Close()
			if _, err2 = io.Copy(dst, src); err2 != nil {
				os.Remove(finalPath)
				return fmt.Errorf("copy to final: %w", err2)
			}
			os.Remove(info.storagePath)
		}
	}

	_, err := h.db.Exec(
		`INSERT INTO files (id, filename, original_name, size, mime_type, storage_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		info.id, originalName, originalName, info.offset, mimeType, finalPath,
	)
	if err != nil {
		os.Remove(finalPath)
		info.storagePath = finalPath
		return fmt.Errorf("save record: %w", err)
	}

	info.storagePath = finalPath
	info.isFinished = true
	slog.Info("tus upload complete", "id", info.id, "name", originalName, "size", info.offset)
	return nil
}

func uploadID(r *http.Request) string {
	id := chi.URLParam(r, "id")
	if id != "" {
		return id
	}
	id = filepath.Base(r.URL.Path)
	if id == "." || id == "/" {
		return ""
	}
	return id
}

func (h *Handler) TusCreateUpload(w http.ResponseWriter, r *http.Request) {
	uploadLengthStr := r.Header.Get("Upload-Length")
	if uploadLengthStr == "" {
		http.Error(w, "missing Upload-Length header", http.StatusBadRequest)
		return
	}
	uploadLength, err := strconv.ParseInt(uploadLengthStr, 10, 64)
	if err != nil || uploadLength < 0 {
		http.Error(w, "invalid Upload-Length", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	now := time.Now()
	dateDir := now.Format("2006-01")
	storageDir := filepath.Join(h.dataDir, "uploads", dateDir, id)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		slog.Error("failed to create upload dir", "id", id, "error", err)
		http.Error(w, "failed to create storage", http.StatusInternalServerError)
		return
	}

	tempPath := filepath.Join(storageDir, ".upload.tmp")
	f, err := os.Create(tempPath)
	if err != nil {
		os.RemoveAll(storageDir)
		slog.Error("failed to create temp file", "id", id, "error", err)
		http.Error(w, "failed to create temp file", http.StatusInternalServerError)
		return
	}
	f.Close()

	metadata := parseUploadMetadata(r.Header.Get("Upload-Metadata"))

	info := &uploadInfo{
		id:          id,
		size:        uploadLength,
		offset:      0,
		metadata:    metadata,
		storagePath: tempPath,
		tempDir:     storageDir,
	}

	if uploadLength == 0 {
		info.mu.Lock()
		err := h.finalizeUpload(info)
		info.mu.Unlock()
		if err != nil {
			os.RemoveAll(storageDir)
			slog.Error("failed to finalize empty upload", "id", id, "error", err)
			http.Error(w, "failed to finalize upload", http.StatusInternalServerError)
			return
		}
	}

	h.uploads.Store(id, info)

	slog.Info("tus upload created", "id", id, "size", uploadLength, "metadata", metadata)

	location := r.URL.Path + "/" + id
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) TusHeadUpload(w http.ResponseWriter, r *http.Request) {
	id := uploadID(r)
	if id == "" {
		http.Error(w, "missing upload id", http.StatusBadRequest)
		return
	}

	value, ok := h.uploads.Load(id)
	if !ok {
		http.Error(w, "upload not found", http.StatusNotFound)
		return
	}
	info := value.(*uploadInfo)

	info.mu.Lock()
	offset := info.offset
	size := info.size
	isFinished := info.isFinished
	metadata := info.metadata
	info.mu.Unlock()

	w.Header().Set("Upload-Offset", strconv.FormatInt(offset, 10))
	if size >= 0 {
		w.Header().Set("Upload-Length", strconv.FormatInt(size, 10))
	}
	if isFinished {
		w.Header().Set("Upload-Complete", "true")
	}
	if encoded := encodeUploadMetadata(metadata); encoded != "" {
		w.Header().Set("Upload-Metadata", encoded)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) TusPatchUpload(w http.ResponseWriter, r *http.Request) {
	id := uploadID(r)
	if id == "" {
		http.Error(w, "missing upload id", http.StatusBadRequest)
		return
	}

	if r.Header.Get("Content-Type") != "application/offset+octet-stream" {
		http.Error(w, "invalid Content-Type, expected application/offset+octet-stream", http.StatusBadRequest)
		return
	}

	uploadOffsetStr := r.Header.Get("Upload-Offset")
	if uploadOffsetStr == "" {
		http.Error(w, "missing Upload-Offset header", http.StatusBadRequest)
		return
	}
	requestOffset, err := strconv.ParseInt(uploadOffsetStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid Upload-Offset", http.StatusBadRequest)
		return
	}

	value, ok := h.uploads.Load(id)
	if !ok {
		http.Error(w, "upload not found", http.StatusNotFound)
		return
	}
	info := value.(*uploadInfo)

	info.mu.Lock()
	defer info.mu.Unlock()

	if info.isFinished {
		http.Error(w, "upload already finished", http.StatusGone)
		return
	}

	if requestOffset != info.offset {
		http.Error(w, fmt.Sprintf("offset mismatch: expected %d, got %d", info.offset, requestOffset), http.StatusConflict)
		return
	}

	f, err := os.OpenFile(info.storagePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Error("failed to open temp file", "id", id, "error", err)
		http.Error(w, "failed to open file", http.StatusInternalServerError)
		return
	}

	written, err := io.Copy(f, r.Body)
	if err != nil {
		f.Close()
		slog.Error("failed to write chunk", "id", id, "error", err)
		http.Error(w, "failed to write chunk", http.StatusInternalServerError)
		return
	}
	f.Close()

	info.offset += written

	if info.size > 0 && info.offset >= info.size {
		if err := h.finalizeUpload(info); err != nil {
			slog.Error("failed to finalize upload", "id", id, "error", err)
			http.Error(w, "failed to finalize upload: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Upload-Offset", strconv.FormatInt(info.offset, 10))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TusDeleteUpload(w http.ResponseWriter, r *http.Request) {
	id := uploadID(r)
	if id == "" {
		http.Error(w, "missing upload id", http.StatusBadRequest)
		return
	}

	value, ok := h.uploads.Load(id)
	if !ok {
		http.Error(w, "upload not found", http.StatusNotFound)
		return
	}
	info := value.(*uploadInfo)

	info.mu.Lock()
	info.mu.Unlock()

	if info.tempDir != "" {
		os.RemoveAll(info.tempDir)
	}

	h.uploads.Delete(id)
	slog.Info("tus upload cancelled", "id", id)
	w.WriteHeader(http.StatusNoContent)
}


