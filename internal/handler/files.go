package handler

import (
	"database/sql"
	"net/http"
	"os"
	"strconv"

	"fls/internal/model"

	"github.com/go-chi/chi/v5"
)

type FileHandler struct {
	db *sql.DB
}

func NewFileHandler(db *sql.DB) *FileHandler {
	return &FileHandler{db: db}
}

type fileRow struct {
	model.File
	ShareCount int
}

type filesPageData struct {
	Authenticated bool
	Files         []fileRow
	Search        string
	Page          int
	TotalPages    int
	PrevPage      int
	NextPage      int
}

type fileDetailPageData struct {
	Authenticated bool
	File          model.File
	Shares        []model.Share
	EditMode      bool
}

func (h *FileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	pageStr := r.URL.Query().Get("page")
	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 1 {
		page = p
	}
	perPage := 20

	var total int
	countQuery := "SELECT COUNT(*) FROM files"
	countArgs := []interface{}{}
	if search != "" {
		countQuery += " WHERE filename LIKE ?"
		countArgs = append(countArgs, "%"+search+"%")
	}
	if err := h.db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}
	offset := (page - 1) * perPage

	listQuery := `SELECT f.id, f.filename, f.original_name, f.size, f.mime_type, f.storage_path, f.created_at, f.updated_at,
		(SELECT COUNT(*) FROM shares WHERE file_id = f.id) as share_count
		FROM files f`
	listArgs := []interface{}{}
	if search != "" {
		listQuery += " WHERE f.filename LIKE ?"
		listArgs = append(listArgs, "%"+search+"%")
	}
	listQuery += " ORDER BY f.created_at DESC LIMIT ? OFFSET ?"
	listArgs = append(listArgs, perPage, offset)

	rows, err := h.db.Query(listQuery, listArgs...)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var files []fileRow
	for rows.Next() {
		var f fileRow
		if err := rows.Scan(&f.ID, &f.Filename, &f.OriginalName, &f.Size, &f.MimeType, &f.StoragePath, &f.CreatedAt, &f.UpdatedAt, &f.ShareCount); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		files = append(files, f)
	}
	if files == nil {
		files = []fileRow{}
	}

	prevPage := page - 1
	if prevPage < 1 {
		prevPage = 1
	}
	nextPage := page + 1
	if nextPage > totalPages {
		nextPage = totalPages
	}

	RenderTemplate(w, "files", filesPageData{
		Authenticated: true,
		Files:         files,
		Search:        search,
		Page:          page,
		TotalPages:    totalPages,
		PrevPage:      prevPage,
		NextPage:      nextPage,
	})
}

func (h *FileHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var f model.File
	err := h.db.QueryRow(
		`SELECT id, filename, original_name, size, mime_type, storage_path, created_at, updated_at FROM files WHERE id = ?`, id,
	).Scan(&f.ID, &f.Filename, &f.OriginalName, &f.Size, &f.MimeType, &f.StoragePath, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	shares, err := h.getFileShares(id)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	RenderTemplate(w, "file-detail", fileDetailPageData{
		Authenticated: true,
		File:          f,
		Shares:        shares,
		EditMode:      false,
	})
}

func (h *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var storagePath string
	err := h.db.QueryRow("SELECT storage_path FROM files WHERE id = ?", id).Scan(&storagePath)
	if err == sql.ErrNoRows {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	if storagePath != "" {
		os.Remove(storagePath)
	}

	if _, err := h.db.Exec("DELETE FROM files WHERE id = ?", id); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/admin/files")
	w.WriteHeader(http.StatusOK)
}

func (h *FileHandler) EditFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var f model.File
	err := h.db.QueryRow(
		`SELECT id, filename, original_name, size, mime_type, storage_path, created_at, updated_at FROM files WHERE id = ?`, id,
	).Scan(&f.ID, &f.Filename, &f.OriginalName, &f.Size, &f.MimeType, &f.StoragePath, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	shares, err := h.getFileShares(id)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	RenderTemplate(w, "file-detail", fileDetailPageData{
		Authenticated: true,
		File:          f,
		Shares:        shares,
		EditMode:      true,
	})
}

func (h *FileHandler) UpdateFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	originalName := r.FormValue("original_name")
	if originalName == "" {
		http.Error(w, "original_name is required", http.StatusBadRequest)
		return
	}

	result, err := h.db.Exec("UPDATE files SET original_name = ? WHERE id = ?", originalName, id)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	http.Redirect(w, r, "/admin/files/"+id, http.StatusSeeOther)
}

func (h *FileHandler) getFileShares(fileID string) ([]model.Share, error) {
	rows, err := h.db.Query(
		`SELECT id, file_id, token, password_hash, expires_at, max_downloads, download_count, content_type, text_content, created_at, updated_at FROM shares WHERE file_id = ?`, fileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []model.Share
	for rows.Next() {
		var s model.Share
		if err := rows.Scan(&s.ID, &s.FileID, &s.Token, &s.PasswordHash, &s.ExpiresAt, &s.MaxDownloads, &s.DownloadCount, &s.ContentType, &s.TextContent, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}
	if shares == nil {
		shares = []model.Share{}
	}
	return shares, nil
}
