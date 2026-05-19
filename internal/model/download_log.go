package model

import "time"

type DownloadLog struct {
	ID           string    `json:"id"`
	ShareID      string    `json:"share_id"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	DownloadedAt time.Time `json:"downloaded_at"`
}
