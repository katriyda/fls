package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
}

func TestMigrate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	tables := []string{"config", "download_logs", "files", "shares"}
	for _, name := range tables {
		var exists int
		err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
		if exists != 1 {
			t.Errorf("table %s not found", name)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestInsertAndQueryFile(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	id := "test-file-id"
	filename := "test.txt"
	originalName := "original.txt"
	size := int64(1024)
	mimeType := "text/plain"
	storagePath := "/tmp/test.txt"

	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, mime_type, storage_path) VALUES (?, ?, ?, ?, ?, ?)`,
		id, filename, originalName, size, mimeType, storagePath,
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	var got struct {
		id, filename, originalName, mimeType, storagePath string
		size                                             int64
	}

	err = db.QueryRow(
		`SELECT id, filename, original_name, size, mime_type, storage_path FROM files WHERE id = ?`, id,
	).Scan(&got.id, &got.filename, &got.originalName, &got.size, &got.mimeType, &got.storagePath)
	if err != nil {
		t.Fatalf("query file: %v", err)
	}

	if got.id != id {
		t.Errorf("id = %q, want %q", got.id, id)
	}
	if got.filename != filename {
		t.Errorf("filename = %q, want %q", got.filename, filename)
	}
	if got.originalName != originalName {
		t.Errorf("originalName = %q, want %q", got.originalName, originalName)
	}
	if got.size != size {
		t.Errorf("size = %d, want %d", got.size, size)
	}
	if got.mimeType != mimeType {
		t.Errorf("mimeType = %q, want %q", got.mimeType, mimeType)
	}
	if got.storagePath != storagePath {
		t.Errorf("storagePath = %q, want %q", got.storagePath, storagePath)
	}
}

func TestInsertDuplicateFile(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	id := "dup-file-id"
	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		id, "a.txt", "a.txt", 100, "/tmp/a.txt",
	)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		id, "b.txt", "b.txt", 200, "/tmp/b.txt",
	)
	if err == nil {
		t.Fatal("expected error on duplicate primary key")
	}
}

func TestNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var name string
	err := db.QueryRow(`SELECT filename FROM files WHERE id = ?`, "nonexistent").Scan(&name)
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestShareForeignKey(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	_, err := db.Exec(
		`INSERT INTO shares (id, token, file_id) VALUES (?, ?, ?)`,
		"share-1", "tok-1", "nonexistent-file",
	)
	if err == nil {
		t.Fatal("expected foreign key constraint violation error, got nil")
	}
}

func TestConfigCRUD(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	_, err := db.Exec(`INSERT INTO config (key, value) VALUES (?, ?)`, "max_upload_size", "10485760")
	if err != nil {
		t.Fatalf("insert config: %v", err)
	}

	var val string
	err = db.QueryRow(`SELECT value FROM config WHERE key = ?`, "max_upload_size").Scan(&val)
	if err != nil {
		t.Fatalf("query config: %v", err)
	}
	if val != "10485760" {
		t.Errorf("value = %q, want %q", val, "10485760")
	}

	_, err = db.Exec(`UPDATE config SET value = ? WHERE key = ?`, "20971520", "max_upload_size")
	if err != nil {
		t.Fatalf("update config: %v", err)
	}

	err = db.QueryRow(`SELECT value FROM config WHERE key = ?`, "max_upload_size").Scan(&val)
	if err != nil {
		t.Fatalf("query updated config: %v", err)
	}
	if val != "20971520" {
		t.Errorf("value = %q, want %q", val, "20971520")
	}

	_, err = db.Exec(`DELETE FROM config WHERE key = ?`, "max_upload_size")
	if err != nil {
		t.Fatalf("delete config: %v", err)
	}

	err = db.QueryRow(`SELECT value FROM config WHERE key = ?`, "max_upload_size").Scan(&val)
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows after delete, got %v", err)
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := New(path)
	if err != nil {
		t.Fatalf("New(%q) error = %v", path, err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
