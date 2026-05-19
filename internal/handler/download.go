package handler

import (
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"fls/internal/model"
	"fls/internal/service"

	"github.com/go-chi/chi/v5"
)

type DownloadHandler struct {
	db       *sql.DB
	shareSvc *service.ShareService
	statsSvc *service.StatsService
	verified sync.Map
}

func NewDownloadHandler(db *sql.DB, shareSvc *service.ShareService, statsSvc *service.StatsService) *DownloadHandler {
	return &DownloadHandler{db: db, shareSvc: shareSvc, statsSvc: statsSvc}
}

func (h *DownloadHandler) isVerified(token string) bool {
	v, ok := h.verified.Load(token)
	if !ok {
		return false
	}
	expiry, ok := v.(time.Time)
	if !ok || time.Now().After(expiry) {
		h.verified.Delete(token)
		return false
	}
	return true
}

func (h *DownloadHandler) markVerified(token string) {
	h.verified.Store(token, time.Now().Add(24*time.Hour))
}

func (h *DownloadHandler) ServeShare(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	share, err := h.shareSvc.GetShareByToken(token)
	if err != nil {
		http.Error(w, "share not found", http.StatusNotFound)
		return
	}

	if share.IsExpired() {
		RenderTemplate(w, "download-expired", map[string]interface{}{
			"Authenticated": false,
			"Token":         token,
		})
		return
	}

	if share.PasswordHash != "" && !h.isVerified(token) {
		RenderTemplate(w, "download-password", map[string]interface{}{
			"Authenticated": false,
			"Token":         token,
			"Error":         "",
		})
		return
	}

	if share.IsTextShare() {
		RenderTemplate(w, "download-text", map[string]interface{}{
			"Authenticated": false,
			"Token":         token,
			"TextContent":   share.TextContent,
			"Size":          int64(len(share.TextContent)),
		})
		return
	}

	if share.FileID == nil {
		http.Error(w, "invalid share", http.StatusInternalServerError)
		return
	}

	file, err := h.getFile(*share.FileID)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	mimeType := file.MimeType
	if len(mimeType) >= 6 && mimeType[:6] == "image/" {
		RenderTemplate(w, "download-image", map[string]interface{}{
			"Authenticated": false,
			"Token":         token,
			"FileName":      file.OriginalName,
			"Size":          file.Size,
			"MimeType":      file.MimeType,
		})
		return
	}

	RenderTemplate(w, "download", map[string]interface{}{
		"Authenticated": false,
		"Token":         token,
		"FileName":      file.OriginalName,
		"Size":          file.Size,
		"MimeType":      file.MimeType,
	})
}

func (h *DownloadHandler) VerifySharePassword(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	share, err := h.shareSvc.GetShareByToken(token)
	if err != nil {
		http.Error(w, "share not found", http.StatusNotFound)
		return
	}

	if share.IsExpired() {
		RenderTemplate(w, "download-expired", map[string]interface{}{
			"Authenticated": false,
			"Token":         token,
		})
		return
	}

	if err := r.ParseForm(); err != nil {
		RenderTemplate(w, "download-password", map[string]interface{}{
			"Authenticated": false,
			"Token":         token,
			"Error":         "invalid form data",
		})
		return
	}

	password := r.FormValue("password")
	if share.IsPasswordCorrect(password) {
		h.markVerified(token)
		http.Redirect(w, r, "/s/"+token, http.StatusSeeOther)
		return
	}

	RenderTemplate(w, "download-password", map[string]interface{}{
		"Authenticated": false,
		"Token":         token,
		"Error":         "密码错误",
	})
}

func (h *DownloadHandler) RawContent(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	share, err := h.shareSvc.GetShareByToken(token)
	if err != nil {
		http.Error(w, "share not found", http.StatusNotFound)
		return
	}

	if share.IsExpired() {
		http.Error(w, "share has expired", http.StatusGone)
		return
	}

	if share.PasswordHash != "" && !h.isVerified(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if share.IsTextShare() {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(share.TextContent))
		return
	}

	if share.FileID == nil {
		http.Error(w, "invalid share", http.StatusInternalServerError)
		return
	}

	file, err := h.getFile(*share.FileID)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	if len(file.MimeType) < 6 || file.MimeType[:6] != "image/" {
		http.Error(w, "not an image", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, file.StoragePath)
}

func (h *DownloadHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	share, err := h.shareSvc.GetShareByToken(token)
	if err != nil {
		http.Error(w, "share not found", http.StatusNotFound)
		return
	}

	if share.IsExpired() {
		http.Error(w, "share has expired", http.StatusGone)
		return
	}

	if share.PasswordHash != "" && !h.isVerified(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var originalName string
	var fileSize int64
	var storagePath string

	if share.IsTextShare() {
		originalName = "content.txt"
		fileSize = int64(len(share.TextContent))
	} else {
		if share.FileID == nil {
			http.Error(w, "invalid share", http.StatusInternalServerError)
			return
		}
		fileObj, err := h.getFile(*share.FileID)
		if err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		originalName = fileObj.OriginalName
		fileSize = fileObj.Size
		storagePath = fileObj.StoragePath
	}

	h.statsSvc.RecordDownload(share.ID, r.RemoteAddr, r.UserAgent())
	h.shareSvc.IncrementDownloadCount(share.ID)

	w.Header().Set("Content-Disposition", "attachment; filename="+originalName)

	if share.IsTextShare() {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
		w.Write([]byte(share.TextContent))
		return
	}

	f, err := os.Open(storagePath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "file error", http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, originalName, stat.ModTime(), f)
}

func (h *DownloadHandler) getFile(id string) (*model.File, error) {
	var f model.File
	err := h.db.QueryRow(
		`SELECT id, filename, original_name, size, mime_type, storage_path, created_at, updated_at FROM files WHERE id = ?`, id,
	).Scan(&f.ID, &f.Filename, &f.OriginalName, &f.Size, &f.MimeType, &f.StoragePath, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
