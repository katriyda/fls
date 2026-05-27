package service

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"fls/internal/config"
	"fls/internal/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type ShareService struct {
	db  *sql.DB
	cfg *config.Config
}

func NewShareService(db *sql.DB, cfg *config.Config) *ShareService {
	return &ShareService{db: db, cfg: cfg}
}

func (s *ShareService) GenerateToken(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	token := make([]byte, length)
	for i := range token {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("generate random: %w", err)
		}
		token[i] = charset[idx.Int64()]
	}
	return string(token), nil
}

func (s *ShareService) generateUniqueToken(length int) (string, error) {
	for i := 0; i < 10; i++ {
		token, err := s.GenerateToken(length)
		if err != nil {
			return "", err
		}
		var count int
		err = s.db.QueryRow("SELECT COUNT(*) FROM shares WHERE token = ?", token).Scan(&count)
		if err != nil {
			return "", fmt.Errorf("check token uniqueness: %w", err)
		}
		if count == 0 {
			return token, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique token after 10 attempts")
}

func (s *ShareService) CreateFileShare(fileID, passwordHash string, expiresAt *time.Time, maxDownloads int) (*model.Share, error) {
	id := uuid.New().String()
	length := 8
	if s.cfg != nil {
		if cfgLen := s.cfg.GetTokenLength(); cfgLen >= 4 {
			length = cfgLen
		}
	}
	token, err := s.generateUniqueToken(length)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	var passwordHashOrNil *string
	if passwordHash != "" {
		passwordHashOrNil = &passwordHash
	}

	_, err = s.db.Exec(
		`INSERT INTO shares (id, file_id, token, password_hash, expires_at, max_downloads, download_count, content_type, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0, 'file', ?, ?)`,
		id, fileID, token, passwordHashOrNil, expiresAt, maxDownloads, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create file share: %w", err)
	}

	share := &model.Share{
		ID:            id,
		Token:         token,
		PasswordHash:  passwordHash,
		ExpiresAt:     expiresAt,
		MaxDownloads:  maxDownloads,
		DownloadCount: 0,
		ContentType:   "file",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	share.FileID = &fileID

	return share, nil
}

func (s *ShareService) CreateTextShare(textContent, passwordHash string, expiresAt *time.Time, maxDownloads int) (*model.Share, error) {
	id := uuid.New().String()
	length := 8
	if s.cfg != nil {
		if cfgLen := s.cfg.GetTokenLength(); cfgLen >= 4 {
			length = cfgLen
		}
	}
	token, err := s.generateUniqueToken(length)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	var passwordHashOrNil *string
	if passwordHash != "" {
		passwordHashOrNil = &passwordHash
	}

	_, err = s.db.Exec(
		`INSERT INTO shares (id, token, password_hash, expires_at, max_downloads, download_count, content_type, text_content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0, 'text', ?, ?, ?)`,
		id, token, passwordHashOrNil, expiresAt, maxDownloads, textContent, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create text share: %w", err)
	}

	return &model.Share{
		ID:            id,
		Token:         token,
		PasswordHash:  passwordHash,
		ExpiresAt:     expiresAt,
		MaxDownloads:  maxDownloads,
		DownloadCount: 0,
		ContentType:   "text",
		TextContent:   textContent,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func scanShare(row interface{ Scan(dest ...interface{}) error }) (*model.Share, error) {
	var share model.Share
	var fileID, passwordHash, textContent, originalName sql.NullString
	var expiresAt, featuredAt sql.NullTime
	var isFeatured sql.NullInt64

	err := row.Scan(
		&share.ID, &fileID, &share.Token, &passwordHash, &expiresAt,
		&share.MaxDownloads, &share.DownloadCount, &share.ContentType, &textContent,
		&share.CreatedAt, &share.UpdatedAt, &originalName,
		&isFeatured, &featuredAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("share not found")
		}
		return nil, fmt.Errorf("scan share: %w", err)
	}

	if fileID.Valid {
		share.FileID = &fileID.String
	}
	if passwordHash.Valid {
		share.PasswordHash = passwordHash.String
	}
	if expiresAt.Valid {
		share.ExpiresAt = &expiresAt.Time
	}
	share.TextContent = textContent.String
	if originalName.Valid {
		share.FileName = originalName.String
	}
	share.IsFeatured = isFeatured.Valid && isFeatured.Int64 == 1
	if featuredAt.Valid {
		share.FeaturedAt = &featuredAt.Time
	}

	return &share, nil
}

const shareSelectCols = `s.id, s.file_id, s.token, s.password_hash, s.expires_at,
	s.max_downloads, s.download_count, s.content_type, s.text_content,
	s.created_at, s.updated_at, f.original_name,
	s.is_featured, s.featured_at`

func (s *ShareService) GetShare(id string) (*model.Share, error) {
	return scanShare(s.db.QueryRow(
		`SELECT `+shareSelectCols+`
		 FROM shares s LEFT JOIN files f ON s.file_id = f.id WHERE s.id = ?`, id,
	))
}

func (s *ShareService) GetShareByToken(token string) (*model.Share, error) {
	return scanShare(s.db.QueryRow(
		`SELECT `+shareSelectCols+`
		 FROM shares s LEFT JOIN files f ON s.file_id = f.id WHERE s.token = ?`, token,
	))
}

func (s *ShareService) ListShares(offset, limit int) ([]*model.Share, int, error) {
	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM shares").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count shares: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT `+shareSelectCols+`
		 FROM shares s LEFT JOIN files f ON s.file_id = f.id ORDER BY s.created_at DESC LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list shares: %w", err)
	}
	defer rows.Close()

	var shares []*model.Share
	for rows.Next() {
		share, err := scanShare(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan share row: %w", err)
		}
		shares = append(shares, share)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate shares: %w", err)
	}

	return shares, total, nil
}

func (s *ShareService) ListFeaturedShares() ([]*model.Share, error) {
	rows, err := s.db.Query(
		`SELECT `+shareSelectCols+`
		 FROM shares s LEFT JOIN files f ON s.file_id = f.id
		 WHERE s.is_featured = 1
		   AND (s.expires_at IS NULL OR s.expires_at > datetime('now'))
		   AND (s.max_downloads = 0 OR s.download_count < s.max_downloads)
		 ORDER BY s.featured_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list featured shares: %w", err)
	}
	defer rows.Close()

	var shares []*model.Share
	for rows.Next() {
		share, err := scanShare(rows)
		if err != nil {
			return nil, fmt.Errorf("scan featured share row: %w", err)
		}
		shares = append(shares, share)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate featured shares: %w", err)
	}

	return shares, nil
}

func (s *ShareService) SetFeatured(id string, featured bool) error {
	var featuredAt *time.Time
	if featured {
		t := time.Now()
		featuredAt = &t
	}
	_, err := s.db.Exec(
		"UPDATE shares SET is_featured = ?, featured_at = ?, updated_at = ? WHERE id = ?",
		featured, featuredAt, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("set featured: %w", err)
	}
	return nil
}

// TryIncrementDownloadCount atomically increments the download count only if the limit has not been reached.
// Returns true if the increment succeeded (limit not reached), false if the limit was already reached.
func (s *ShareService) TryIncrementDownloadCount(shareID string) (bool, error) {
	result, err := s.db.Exec(
		"UPDATE shares SET download_count = download_count + 1 WHERE id = ? AND (max_downloads = 0 OR download_count < max_downloads)",
		shareID,
	)
	if err != nil {
		return false, fmt.Errorf("try increment download count: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check rows affected: %w", err)
	}
	return affected > 0, nil
}

func (s *ShareService) DeleteShare(id string) error {
	result, err := s.db.Exec("DELETE FROM shares WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete share: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("share not found")
	}
	return nil
}

func (s *ShareService) GetFileShares(fileID string) ([]*model.Share, error) {
	rows, err := s.db.Query(
		`SELECT `+shareSelectCols+`
		 FROM shares s LEFT JOIN files f ON s.file_id = f.id WHERE s.file_id = ? ORDER BY s.created_at DESC`, fileID,
	)
	if err != nil {
		return nil, fmt.Errorf("get file shares: %w", err)
	}
	defer rows.Close()

	var shares []*model.Share
	for rows.Next() {
		share, err := scanShare(rows)
		if err != nil {
			return nil, fmt.Errorf("scan share row: %w", err)
		}
		shares = append(shares, share)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate file shares: %w", err)
	}

	return shares, nil
}

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}
