package handler

import (
	"database/sql"
	"net/http"
	"time"

	"fls/internal/service"
)

type DashboardHandler struct {
	statsSvc  *service.StatsService
	shareSvc  *service.ShareService
	db        *sql.DB
}

func NewDashboardHandler(statsSvc *service.StatsService, shareSvc *service.ShareService, db *sql.DB) *DashboardHandler {
	return &DashboardHandler{statsSvc: statsSvc, shareSvc: shareSvc, db: db}
}

type RecentFileEntry struct {
	ID        string
	Name      string
	Size      int64
	MimeType  string
	CreatedAt time.Time
}

type RecentShareEntry struct {
	ID            string
	Token         string
	ShareType     string
	FileID        *string
	FileName      string
	ExpiresAt     *time.Time
	DownloadCount int
	CreatedAt     time.Time
}

type DashboardData struct {
	Authenticated bool
	Stats         *service.GlobalStats
	RecentFiles   []RecentFileEntry
	RecentShares  []RecentShareEntry
}

func (h *DashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.statsSvc.GetGlobalStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	recentFiles, err := h.getRecentFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	recentShares, err := h.getRecentShares()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	RenderTemplate(w, "dashboard", DashboardData{
		Authenticated: true,
		Stats:         stats,
		RecentFiles:   recentFiles,
		RecentShares:  recentShares,
	})
}

func (h *DashboardHandler) getRecentFiles() ([]RecentFileEntry, error) {
	rows, err := h.db.Query(`SELECT id, filename, size, mime_type, created_at FROM files ORDER BY created_at DESC LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []RecentFileEntry
	for rows.Next() {
		var f RecentFileEntry
		if err := rows.Scan(&f.ID, &f.Name, &f.Size, &f.MimeType, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if files == nil {
		files = []RecentFileEntry{}
	}
	return files, nil
}

func (h *DashboardHandler) getRecentShares() ([]RecentShareEntry, error) {
	rows, err := h.db.Query(`
		SELECT s.id, s.token, s.content_type, s.file_id, s.expires_at,
		       s.download_count, s.created_at, COALESCE(f.original_name, '[text]')
		FROM shares s
		LEFT JOIN files f ON s.file_id = f.id
		ORDER BY s.created_at DESC LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []RecentShareEntry
	for rows.Next() {
		var s RecentShareEntry
		var fileID sql.NullString
		var expiresAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.Token, &s.ShareType, &fileID, &expiresAt, &s.DownloadCount, &s.CreatedAt, &s.FileName); err != nil {
			return nil, err
		}
		if fileID.Valid {
			s.FileID = &fileID.String
		}
		if expiresAt.Valid {
			s.ExpiresAt = &expiresAt.Time
		}
		shares = append(shares, s)
	}
	if shares == nil {
		shares = []RecentShareEntry{}
	}
	return shares, nil
}
