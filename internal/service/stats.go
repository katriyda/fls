package service

import (
	"database/sql"
	"fmt"

	"fls/internal/model"

	"github.com/google/uuid"
)

type ShareStats struct {
	TotalDownloads  int                 `json:"total_downloads"`
	UniqueIPs       int                 `json:"unique_ips"`
	RecentDownloads []model.DownloadLog `json:"recent_downloads"`
}

type GlobalStats struct {
	TotalFiles     int   `json:"total_files"`
	TotalShares    int   `json:"total_shares"`
	TotalDownloads int   `json:"total_downloads"`
	ActiveShares   int   `json:"active_shares"`
	TotalStorage   int64 `json:"total_storage"`
}

type StatsService struct {
	db *sql.DB
}

func NewStatsService(db *sql.DB) *StatsService {
	return &StatsService{db: db}
}

func (s *StatsService) RecordDownload(shareID, ipAddress, userAgent string) error {
	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT INTO download_logs (id, share_id, ip_address, user_agent, downloaded_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		id, shareID, ipAddress, userAgent,
	)
	if err != nil {
		return fmt.Errorf("record download: %w", err)
	}
	return nil
}

func (s *StatsService) GetShareStats(shareID string) (*ShareStats, error) {
	var totalDownloads int
	err := s.db.QueryRow("SELECT COUNT(*) FROM download_logs WHERE share_id = ?", shareID).Scan(&totalDownloads)
	if err != nil {
		return nil, fmt.Errorf("count share downloads: %w", err)
	}

	var uniqueIPs int
	err = s.db.QueryRow("SELECT COUNT(DISTINCT ip_address) FROM download_logs WHERE share_id = ?", shareID).Scan(&uniqueIPs)
	if err != nil {
		return nil, fmt.Errorf("count unique ips: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT id, share_id, ip_address, user_agent, downloaded_at FROM download_logs WHERE share_id = ? ORDER BY downloaded_at DESC LIMIT 10`,
		shareID,
	)
	if err != nil {
		return nil, fmt.Errorf("query recent downloads: %w", err)
	}
	defer rows.Close()

	recent := make([]model.DownloadLog, 0)
	for rows.Next() {
		var log model.DownloadLog
		if err := rows.Scan(&log.ID, &log.ShareID, &log.IPAddress, &log.UserAgent, &log.DownloadedAt); err != nil {
			return nil, fmt.Errorf("scan download log: %w", err)
		}
		recent = append(recent, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate download logs: %w", err)
	}

	return &ShareStats{
		TotalDownloads:  totalDownloads,
		UniqueIPs:       uniqueIPs,
		RecentDownloads: recent,
	}, nil
}

func (s *StatsService) GetGlobalStats() (*GlobalStats, error) {
	var totalFiles int
	err := s.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&totalFiles)
	if err != nil {
		return nil, fmt.Errorf("count files: %w", err)
	}

	var totalShares int
	err = s.db.QueryRow("SELECT COUNT(*) FROM shares").Scan(&totalShares)
	if err != nil {
		return nil, fmt.Errorf("count shares: %w", err)
	}

	var totalDownloads int
	err = s.db.QueryRow("SELECT COUNT(*) FROM download_logs").Scan(&totalDownloads)
	if err != nil {
		return nil, fmt.Errorf("count downloads: %w", err)
	}

	var activeShares int
	err = s.db.QueryRow("SELECT COUNT(*) FROM shares WHERE (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP) AND (max_downloads = 0 OR download_count < max_downloads)").Scan(&activeShares)
	if err != nil {
		return nil, fmt.Errorf("count active shares: %w", err)
	}

	var totalStorage sql.NullInt64
	err = s.db.QueryRow("SELECT SUM(size) FROM files").Scan(&totalStorage)
	if err != nil {
		return nil, fmt.Errorf("sum file sizes: %w", err)
	}

	return &GlobalStats{
		TotalFiles:     totalFiles,
		TotalShares:    totalShares,
		TotalDownloads: totalDownloads,
		ActiveShares:   activeShares,
		TotalStorage:   totalStorage.Int64,
	}, nil
}
