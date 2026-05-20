package model

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Share struct {
	ID            string     `json:"id"`
	FileID        *string    `json:"file_id,omitempty"`
	Token         string     `json:"token"`
	PasswordHash  string     `json:"-"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	MaxDownloads  int        `json:"max_downloads"`
	DownloadCount int        `json:"download_count"`
	ContentType   string     `json:"content_type"` // "file" or "text"
	TextContent   string     `json:"text_content,omitempty"`
	FileName      string     `json:"file_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (s *Share) IsDownloadLimitReached() bool {
	if s.MaxDownloads <= 0 {
		return false
	}
	return s.DownloadCount >= s.MaxDownloads
}

func (s *Share) IsExpired() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*s.ExpiresAt)
}

func (s *Share) IsPasswordCorrect(password string) bool {
	if s.PasswordHash == "" {
		return true // no password set
	}
	err := bcrypt.CompareHashAndPassword([]byte(s.PasswordHash), []byte(password))
	return err == nil
}

func (s *Share) IsTextShare() bool {
	return s.ContentType == "text"
}

func (s *Share) IsFileShare() bool {
	return s.ContentType == "file"
}
