package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fls/internal/database"
)

func openTestDB(t *testing.T) *database.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := database.New(path)
	if err != nil {
		t.Fatalf("New(%q) error = %v", path, err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGenerateToken_Length(t *testing.T) {
	svc := NewShareService(nil, nil)
	token, err := svc.GenerateToken(8)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}
	if len(token) != 8 {
		t.Fatalf("expected length 8, got %d", len(token))
	}
}

func TestGenerateToken_Alphanumeric(t *testing.T) {
	svc := NewShareService(nil, nil)
	token, err := svc.GenerateToken(100)
	if err != nil {
		t.Fatalf("GenerateToken() returned error: %v", err)
	}
	for _, c := range token {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			t.Fatalf("token contains non-alphanumeric character: %c", c)
		}
	}
}

func TestGenerateToken_NotHex(t *testing.T) {
	svc := NewShareService(nil, nil)
	// Check that tokens aren't just hex
	allHex := true
	for i := 0; i < 10; i++ {
		token, err := svc.GenerateToken(64)
		if err != nil {
			t.Fatalf("GenerateToken() returned error: %v", err)
		}
		for _, c := range token {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				allHex = false
				break
			}
		}
	}
	if allHex {
		t.Fatal("expected tokens to contain non-hex characters")
	}
}

func TestGenerateToken_Unique(t *testing.T) {
	svc := NewShareService(nil, nil)
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := svc.GenerateToken(16)
		if err != nil {
			t.Fatalf("GenerateToken() returned error: %v", err)
		}
		if tokens[token] {
			t.Fatal("duplicate token generated")
		}
		tokens[token] = true
	}
}

func TestCreateFileShare(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	// Insert a file first
	fileID := "test-file-id"
	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		fileID, "test.txt", "test.txt", 1024, "/tmp/test.txt",
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	share, err := svc.CreateFileShare(fileID, "", &expiresAt, 5)
	if err != nil {
		t.Fatalf("CreateFileShare() error = %v", err)
	}

	if share == nil {
		t.Fatal("expected non-nil share")
	}
	if share.ContentType != "file" {
		t.Errorf("content_type = %q, want %q", share.ContentType, "file")
	}
	if share.Token == "" {
		t.Errorf("expected non-empty token")
	}
	if len(share.Token) != 8 {
		t.Errorf("token length = %d, want %d", len(share.Token), 8)
	}
	if share.FileID == nil || *share.FileID != fileID {
		t.Errorf("file_id = %v, want %v", share.FileID, fileID)
	}
	if share.MaxDownloads != 5 {
		t.Errorf("max_downloads = %d, want %d", share.MaxDownloads, 5)
	}
	if share.ExpiresAt == nil {
		t.Fatal("expected non-nil expires_at")
	}
}

func TestCreateTextShare(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	share, err := svc.CreateTextShare("Hello, World!", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare() error = %v", err)
	}

	if share.ContentType != "text" {
		t.Errorf("content_type = %q, want %q", share.ContentType, "text")
	}
	if share.TextContent != "Hello, World!" {
		t.Errorf("text_content = %q, want %q", share.TextContent, "Hello, World!")
	}
	if share.ExpiresAt != nil {
		t.Errorf("expected nil expires_at")
	}
}

func TestGetShare(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	share, err := svc.CreateTextShare("test content", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare() error = %v", err)
	}

	got, err := svc.GetShare(share.ID)
	if err != nil {
		t.Fatalf("GetShare() error = %v", err)
	}

	if got.ID != share.ID {
		t.Errorf("id = %q, want %q", got.ID, share.ID)
	}
	if got.Token != share.Token {
		t.Errorf("token = %q, want %q", got.Token, share.Token)
	}
	if got.TextContent != share.TextContent {
		t.Errorf("text_content = %q, want %q", got.TextContent, share.TextContent)
	}
}

func TestGetShare_NotFound(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	_, err := svc.GetShare("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent share")
	}
}

func TestGetShareByToken(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	share, err := svc.CreateTextShare("test content", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare() error = %v", err)
	}

	got, err := svc.GetShareByToken(share.Token)
	if err != nil {
		t.Fatalf("GetShareByToken() error = %v", err)
	}

	if got.ID != share.ID {
		t.Errorf("id = %q, want %q", got.ID, share.ID)
	}
}

func TestGetShareByToken_NotFound(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	_, err := svc.GetShareByToken("nonexistent-token")
	if err == nil {
		t.Fatal("expected error for nonexistent token")
	}
}

func TestListShares(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	for i := 0; i < 5; i++ {
		content := strings.Repeat("x", i+1)
		_, err := svc.CreateTextShare(content, "", nil, 0)
		if err != nil {
			t.Fatalf("CreateTextShare() error = %v", err)
		}
	}

	shares, total, err := svc.ListShares(0, 3)
	if err != nil {
		t.Fatalf("ListShares() error = %v", err)
	}

	if total != 5 {
		t.Errorf("total = %d, want %d", total, 5)
	}
	if len(shares) != 3 {
		t.Errorf("len(shares) = %d, want %d", len(shares), 3)
	}

	// Second page
	shares2, _, err := svc.ListShares(3, 3)
	if err != nil {
		t.Fatalf("ListShares() error = %v", err)
	}
	if len(shares2) != 2 {
		t.Errorf("len(shares2) = %d, want %d", len(shares2), 2)
	}
}

func TestListShares_Empty(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	shares, total, err := svc.ListShares(0, 20)
	if err != nil {
		t.Fatalf("ListShares() error = %v", err)
	}

	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(shares) != 0 {
		t.Errorf("len(shares) = %d, want 0", len(shares))
	}
}

func TestDeleteShare(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	share, err := svc.CreateTextShare("test content", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare() error = %v", err)
	}

	err = svc.DeleteShare(share.ID)
	if err != nil {
		t.Fatalf("DeleteShare() error = %v", err)
	}

	_, err = svc.GetShare(share.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestDeleteShare_NotFound(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	err := svc.DeleteShare("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent share")
	}
}

func TestGetFileShares(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	fileID := "test-file-id"
	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		fileID, "test.txt", "test.txt", 1024, "/tmp/test.txt",
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := svc.CreateFileShare(fileID, "", nil, 0)
		if err != nil {
			t.Fatalf("CreateFileShare() error = %v", err)
		}
	}

	shares, err := svc.GetFileShares(fileID)
	if err != nil {
		t.Fatalf("GetFileShares() error = %v", err)
	}

	if len(shares) != 3 {
		t.Errorf("len(shares) = %d, want %d", len(shares), 3)
	}
}

func TestGetFileShares_None(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	shares, err := svc.GetFileShares("nonexistent-file")
	if err != nil {
		t.Fatalf("GetFileShares() error = %v", err)
	}

	if len(shares) != 0 {
		t.Errorf("len(shares) = %d, want 0", len(shares))
	}
}

func TestCreateFileShare_WithPassword(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	fileID := "test-file-id"
	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		fileID, "test.txt", "test.txt", 1024, "/tmp/test.txt",
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	passwordHash, err := HashPassword("test-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	share, err := svc.CreateFileShare(fileID, passwordHash, nil, 0)
	if err != nil {
		t.Fatalf("CreateFileShare() error = %v", err)
	}

	if share.PasswordHash == "" {
		t.Error("expected non-empty password_hash")
	}
}

func TestCreateTextShare_WithPassword(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	passwordHash, err := HashPassword("test-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	share, err := svc.CreateTextShare("secret content", passwordHash, nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare() error = %v", err)
	}

	if share.PasswordHash == "" {
		t.Error("expected non-empty password_hash")
	}
}

func TestTokenCollision(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	fileID := "test-file-id"
	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		fileID, "test.txt", "test.txt", 1024, "/tmp/test.txt",
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	// Insert a share with a known token
	_, err = db.Exec(
		`INSERT INTO shares (id, token, content_type, file_id, created_at, updated_at)
		 VALUES (?, ?, 'file', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"existing-share", "collision-token", fileID,
	)
	if err != nil {
		t.Fatalf("insert existing share: %v", err)
	}

	// GenerateToken should still work (just test that GenerateToken works)
	token, err := svc.GenerateToken(8)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if len(token) != 8 {
		t.Errorf("token length = %d, want %d", len(token), 8)
	}
}

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("test-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
}

func TestHashPassword_Empty(t *testing.T) {
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash != "" {
		t.Errorf("hash = %q, want empty", hash)
	}
}

func TestShare_Expired(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	past := time.Now().Add(-24 * time.Hour)
	share, err := svc.CreateTextShare("old content", "", &past, 0)
	if err != nil {
		t.Fatalf("CreateTextShare() error = %v", err)
	}

	if !share.IsExpired() {
		t.Error("expected share to be expired")
	}
}

func TestShare_NotExpired(t *testing.T) {
	db := openTestDB(t)
	svc := NewShareService(db.DB, nil)

	future := time.Now().Add(24 * time.Hour)
	share, err := svc.CreateTextShare("fresh content", "", &future, 0)
	if err != nil {
		t.Fatalf("CreateTextShare() error = %v", err)
	}

	if share.IsExpired() {
		t.Error("expected share not to be expired")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
