package handler

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fls/internal/config"
	"fls/internal/middleware"
	"fls/internal/model"
	"fls/internal/service"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
	"golang.org/x/crypto/bcrypt"
)

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

type ShareHandler struct {
	db       *sql.DB
	shareSvc *service.ShareService
	cfg      *config.Config
}

func NewShareHandler(db *sql.DB, shareSvc *service.ShareService, cfg *config.Config) *ShareHandler {
	return &ShareHandler{db: db, shareSvc: shareSvc, cfg: cfg}
}

type shareRow struct {
	*model.Share
	FileName    string
	TextPreview string
	HasPassword bool
	IsFeatured  bool
}

func (h *ShareHandler) ListShares(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	shares, total, err := h.shareSvc.ListShares(offset, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]shareRow, len(shares))
	for i, share := range shares {
		textPreview := ""
		if share.IsTextShare() {
			textPreview = truncateText(share.TextContent, 50)
		}
		rows[i] = shareRow{
			Share:       share,
			FileName:    share.FileName,
			TextPreview: textPreview,
			HasPassword: share.PasswordHash != "",
			IsFeatured:  share.IsFeatured,
		}
	}

	page := offset/limit + 1
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	prevOffset := offset - limit
	if prevOffset < 0 {
		prevOffset = 0
	}
	nextOffset := offset + limit

	RenderTemplate(w, "shares", map[string]interface{}{
		"Authenticated": true,
		"CSRFToken":     middleware.CSRFToken(r),
		"Shares":        rows,
		"Total":         total,
		"Offset":        offset,
		"Limit":         limit,
		"Page":          page,
		"TotalPages":    totalPages,
		"PrevOffset":    prevOffset,
		"NextOffset":    nextOffset,
	})
}

func (h *ShareHandler) getBaseURL(r *http.Request) string {
	if h.cfg != nil && h.cfg.PublicBaseURL != "" {
		return strings.TrimSuffix(h.cfg.PublicBaseURL, "/")
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func (h *ShareHandler) NewShareForm(w http.ResponseWriter, r *http.Request) {
	files, err := h.listFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defaultExpiryStr := "24h"
	if h.cfg != nil {
		switch h.cfg.DefaultExpiry {
		case time.Hour:
			defaultExpiryStr = "1h"
		case 24 * time.Hour:
			defaultExpiryStr = "24h"
		case 7 * 24 * time.Hour:
			defaultExpiryStr = "7d"
		case 30 * 24 * time.Hour:
			defaultExpiryStr = "30d"
		case 0:
			defaultExpiryStr = "never"
		}
	}

	RenderTemplate(w, "share-detail", map[string]interface{}{
		"Authenticated": true,
		"Mode":          "new",
		"Files":         files,
		"DefaultExpiry": defaultExpiryStr,
		"CSRFToken":     middleware.CSRFToken(r),
	})
}

func (h *ShareHandler) CreateShare(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	contentType := r.FormValue("content_type")
	password := r.FormValue("password")
	expiresIn := r.FormValue("expires_in")
	maxDownloadsStr := r.FormValue("max_downloads")

	maxDownloads, _ := strconv.Atoi(maxDownloadsStr)
	if maxDownloads < 0 {
		maxDownloads = 0
	}

	expiresAt := parseExpiresIn(expiresIn)

	var passwordHash string
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "failed to hash password", http.StatusInternalServerError)
			return
		}
		passwordHash = string(hash)
	}

	switch contentType {
	case "file":
		fileID := r.FormValue("file_id")
		if fileID == "" {
			http.Error(w, "file_id is required for file share", http.StatusBadRequest)
			return
		}
		_, err := h.shareSvc.CreateFileShare(fileID, passwordHash, expiresAt, maxDownloads)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "text":
		textContent := r.FormValue("text_content")
		if strings.TrimSpace(textContent) == "" {
			http.Error(w, "text_content is required for text share", http.StatusBadRequest)
			return
		}
		_, err := h.shareSvc.CreateTextShare(textContent, passwordHash, expiresAt, maxDownloads)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "invalid content_type", http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/admin/shares")
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/admin/shares", http.StatusSeeOther)
	}
}

func (h *ShareHandler) GetShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	share, err := h.shareSvc.GetShare(id)
	if err != nil {
		http.Error(w, "share not found", http.StatusNotFound)
		return
	}

	baseURL := h.getBaseURL(r)
	shareURL := baseURL + "/s/" + share.Token

	RenderTemplate(w, "share-detail", map[string]interface{}{
		"Authenticated": true,
		"Mode":          "detail",
		"Share":         share,
		"ShareURL":      shareURL,
		"FileName":      share.FileName,
		"HasPassword":   share.PasswordHash != "",
		"IsFeatured":    share.IsFeatured,
		"CSRFToken":     middleware.CSRFToken(r),
	})
}

func (h *ShareHandler) DeleteShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.shareSvc.DeleteShare(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/admin/shares")
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/admin/shares", http.StatusSeeOther)
	}
}

func (h *ShareHandler) ToggleFeature(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	share, err := h.shareSvc.GetShare(id)
	if err != nil {
		http.Error(w, "share not found", http.StatusNotFound)
		return
	}

	if err := h.shareSvc.SetFeatured(id, !share.IsFeatured); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/admin/shares/"+id)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/admin/shares/"+id, http.StatusSeeOther)
	}
}

func (h *ShareHandler) QRCode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	share, err := h.shareSvc.GetShare(id)
	if err != nil {
		http.Error(w, "share not found", http.StatusNotFound)
		return
	}

	baseURL := h.getBaseURL(r)
	content := baseURL + "/s/" + share.Token

	qrc, err := qrcode.New(content)
	if err != nil {
		http.Error(w, "failed to generate QR code", http.StatusInternalServerError)
		return
	}

	buf := new(bytes.Buffer)
	wtr := standard.NewWithWriter(
		nopCloser{buf},
		standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
		standard.WithQRWidth(10),
	)
	defer wtr.Close()

	if err := qrc.Save(wtr); err != nil {
		http.Error(w, "failed to save QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.Write(buf.Bytes())
}

func (h *ShareHandler) listFiles() ([]model.File, error) {
	rows, err := h.db.Query("SELECT id, filename, original_name, size, mime_type, storage_path, created_at, updated_at FROM files ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	var files []model.File
	for rows.Next() {
		var f model.File
		if err := rows.Scan(&f.ID, &f.Filename, &f.OriginalName, &f.Size, &f.MimeType, &f.StoragePath, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}
		files = append(files, f)
	}

	return files, nil
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func parseExpiresIn(s string) *time.Time {
	switch s {
	case "", "never":
		return nil
	case "1h":
		t := time.Now().Add(1 * time.Hour)
		return &t
	case "24h":
		t := time.Now().Add(24 * time.Hour)
		return &t
	case "7d":
		t := time.Now().Add(7 * 24 * time.Hour)
		return &t
	case "30d":
		t := time.Now().Add(30 * 24 * time.Hour)
		return &t
	default:
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil
		}
		t := time.Now().Add(d)
		return &t
	}
}
