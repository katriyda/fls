package config

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS config (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestDefaults(t *testing.T) {
	c := Defaults()
	if c.Port != 8080 {
		t.Errorf("expected port 8080, got %d", c.Port)
	}
	if c.DataDir != "./data" {
		t.Errorf("expected data_dir './data', got %s", c.DataDir)
	}
	if c.TokenLength != 8 {
		t.Errorf("expected token_length 8, got %d", c.TokenLength)
	}
	if c.MaxUploadSize != 10*1024*1024*1024 {
		t.Errorf("expected max_upload_size 10GB, got %d", c.MaxUploadSize)
	}
	if c.DefaultExpiry != 7*24*time.Hour {
		t.Errorf("expected default_expiry 7d, got %v", c.DefaultExpiry)
	}
	if c.SessionTimeout != 24*time.Hour {
		t.Errorf("expected session_timeout 24h, got %v", c.SessionTimeout)
	}
	if c.LogRetentionDays != 90 {
		t.Errorf("expected log_retention_days 90, got %d", c.LogRetentionDays)
	}
	if c.RateLimitPerMinute != 60 {
		t.Errorf("expected rate_limit_per_minute 60, got %d", c.RateLimitPerMinute)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	db := testDB(t)
	c := New(db)

	c.Port = 3000
	c.DataDir = "/custom/data"
	c.TokenLength = 12
	c.MaxUploadSize = 5 * 1024 * 1024
	c.DefaultExpiry = 1 * time.Hour
	c.SessionTimeout = 30 * time.Minute
	c.LogRetentionDays = 30
	c.RateLimitPerMinute = 100

	if err := c.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	c2 := New(db)
	if c2.Port != 3000 {
		t.Errorf("expected port 3000, got %d", c2.Port)
	}
	if c2.DataDir != "/custom/data" {
		t.Errorf("expected data_dir '/custom/data', got %s", c2.DataDir)
	}
	if c2.TokenLength != 12 {
		t.Errorf("expected token_length 12, got %d", c2.TokenLength)
	}
	if c2.MaxUploadSize != 5*1024*1024 {
		t.Errorf("expected max_upload_size 5MB, got %d", c2.MaxUploadSize)
	}
	if c2.DefaultExpiry != 1*time.Hour {
		t.Errorf("expected default_expiry 1h, got %v", c2.DefaultExpiry)
	}
	if c2.SessionTimeout != 30*time.Minute {
		t.Errorf("expected session_timeout 30m, got %v", c2.SessionTimeout)
	}
	if c2.LogRetentionDays != 30 {
		t.Errorf("expected log_retention_days 30, got %d", c2.LogRetentionDays)
	}
	if c2.RateLimitPerMinute != 100 {
		t.Errorf("expected rate_limit_per_minute 100, got %d", c2.RateLimitPerMinute)
	}
}

func TestApplyOverrides(t *testing.T) {
	c := Defaults()
	c.ApplyOverrides(9090, "/override/data")
	if c.Port != 9090 {
		t.Errorf("expected port 9090, got %d", c.Port)
	}
	if c.DataDir != "/override/data" {
		t.Errorf("expected data_dir '/override/data', got %s", c.DataDir)
	}
}

func TestApplyOverridesZeroValues(t *testing.T) {
	c := Defaults()
	c.Port = 9090
	c.DataDir = "/existing/data"
	c.ApplyOverrides(0, "")
	if c.Port != 9090 {
		t.Errorf("expected port unchanged 9090, got %d", c.Port)
	}
	if c.DataDir != "/existing/data" {
		t.Errorf("expected data_dir unchanged, got %s", c.DataDir)
	}
}

func TestSaveWithoutDB(t *testing.T) {
	c := Defaults()
	err := c.Save()
	if err == nil {
		t.Fatal("expected error when saving without db")
	}
}

func TestLoadWithoutDB(t *testing.T) {
	c := Defaults()
	c.Port = 9999
	c.Load()
	if c.Port != 9999 {
		t.Errorf("expected port unchanged 9999, got %d", c.Port)
	}
}

func TestPartialConfig(t *testing.T) {
	db := testDB(t)

	_, err := db.Exec(`INSERT INTO config (key, value) VALUES ('port', '1234')`)
	if err != nil {
		t.Fatal(err)
	}

	c := New(db)
	if c.Port != 1234 {
		t.Errorf("expected port 1234, got %d", c.Port)
	}
	if c.DataDir != "./data" {
		t.Errorf("expected data_dir default './data', got %s", c.DataDir)
	}
	if c.TokenLength != 8 {
		t.Errorf("expected token_length default 8, got %d", c.TokenLength)
	}
}

func TestPersistAcrossMultipleSaves(t *testing.T) {
	db := testDB(t)

	c1 := New(db)
	c1.Port = 3000
	c1.DataDir = "/first"
	if err := c1.Save(); err != nil {
		t.Fatal(err)
	}

	c2 := New(db)
	c2.Port = 4000
	c2.DataDir = "/second"
	if err := c2.Save(); err != nil {
		t.Fatal(err)
	}

	c3 := New(db)
	if c3.Port != 4000 {
		t.Errorf("expected port 4000, got %d", c3.Port)
	}
	if c3.DataDir != "/second" {
		t.Errorf("expected data_dir '/second', got %s", c3.DataDir)
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("FLS_PORT", "5000")
	t.Setenv("FLS_DATA_DIR", "/env/data")
	t.Setenv("FLS_TOKEN_LENGTH", "16")
	t.Setenv("FLS_MAX_UPLOAD_SIZE", "1048576")
	t.Setenv("FLS_DEFAULT_EXPIRY", "2h")
	t.Setenv("FLS_SESSION_TIMEOUT", "1h")
	t.Setenv("FLS_LOG_RETENTION_DAYS", "7")
	t.Setenv("FLS_RATE_LIMIT_PER_MINUTE", "200")

	c := Defaults()
	c.EnvOverrides()
	if c.Port != 5000 {
		t.Errorf("expected port 5000, got %d", c.Port)
	}
	if c.DataDir != "/env/data" {
		t.Errorf("expected data_dir '/env/data', got %s", c.DataDir)
	}
	if c.TokenLength != 16 {
		t.Errorf("expected token_length 16, got %d", c.TokenLength)
	}
	if c.MaxUploadSize != 1048576 {
		t.Errorf("expected max_upload_size 1048576, got %d", c.MaxUploadSize)
	}
	if c.DefaultExpiry != 2*time.Hour {
		t.Errorf("expected default_expiry 2h, got %v", c.DefaultExpiry)
	}
	if c.SessionTimeout != 1*time.Hour {
		t.Errorf("expected session_timeout 1h, got %v", c.SessionTimeout)
	}
	if c.LogRetentionDays != 7 {
		t.Errorf("expected log_retention_days 7, got %d", c.LogRetentionDays)
	}
	if c.RateLimitPerMinute != 200 {
		t.Errorf("expected rate_limit_per_minute 200, got %d", c.RateLimitPerMinute)
	}
}

func TestNewFromDBLoadsDefaultsForMissingKeys(t *testing.T) {
	db := testDB(t)

	_, err := db.Exec(`INSERT INTO config (key, value) VALUES ('port', '9999')`)
	if err != nil {
		t.Fatal(err)
	}

	c := New(db)
	if c.Port != 9999 {
		t.Errorf("expected port 9999, got %d", c.Port)
	}
	if c.TokenLength != 8 {
		t.Errorf("expected token_length default 8, got %d", c.TokenLength)
	}
	if c.LogRetentionDays != 90 {
		t.Errorf("expected log_retention_days default 90, got %d", c.LogRetentionDays)
	}
}
