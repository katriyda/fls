package service

import (
	"testing"
)

func TestStatsService_RecordDownload(t *testing.T) {
	db := openTestDB(t)
	svc := NewStatsService(db.DB)
	shareSvc := NewShareService(db.DB)

	share, err := shareSvc.CreateTextShare("test content", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	err = svc.RecordDownload(share.ID, "192.168.1.1", "test-agent")
	if err != nil {
		t.Fatalf("RecordDownload: %v", err)
	}

	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM download_logs WHERE share_id = ?", share.ID).Scan(&count)
	if err != nil {
		t.Fatalf("query download_logs: %v", err)
	}
	if count != 1 {
		t.Errorf("download_logs count = %d, want 1", count)
	}
}

func TestStatsService_GetShareStats(t *testing.T) {
	db := openTestDB(t)
	svc := NewStatsService(db.DB)
	shareSvc := NewShareService(db.DB)

	share, err := shareSvc.CreateTextShare("stats test", "", nil, 0)
	if err != nil {
		t.Fatalf("CreateTextShare: %v", err)
	}

	for i := 0; i < 3; i++ {
		ip := "192.168.1.1"
		if i == 2 {
			ip = "192.168.1.2"
		}
		err := svc.RecordDownload(share.ID, ip, "agent")
		if err != nil {
			t.Fatalf("RecordDownload: %v", err)
		}
	}

	stats, err := svc.GetShareStats(share.ID)
	if err != nil {
		t.Fatalf("GetShareStats: %v", err)
	}

	if stats.TotalDownloads != 3 {
		t.Errorf("TotalDownloads = %d, want 3", stats.TotalDownloads)
	}
	if stats.UniqueIPs != 2 {
		t.Errorf("UniqueIPs = %d, want 2", stats.UniqueIPs)
	}
	if len(stats.RecentDownloads) != 3 {
		t.Errorf("len(RecentDownloads) = %d, want 3", len(stats.RecentDownloads))
	}
}

func TestStatsService_GetGlobalStats(t *testing.T) {
	db := openTestDB(t)
	svc := NewStatsService(db.DB)
	shareSvc := NewShareService(db.DB)

	fileID := "global-stats-test-file"
	_, err := db.Exec(
		`INSERT INTO files (id, filename, original_name, size, storage_path) VALUES (?, ?, ?, ?, ?)`,
		fileID, "test.txt", "test.txt", 1024, "/tmp/test.txt",
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	share, err := shareSvc.CreateFileShare(fileID, "", nil, 0)
	if err != nil {
		t.Fatalf("CreateFileShare: %v", err)
	}

	err = svc.RecordDownload(share.ID, "1.2.3.4", "agent")
	if err != nil {
		t.Fatalf("RecordDownload: %v", err)
	}

	stats, err := svc.GetGlobalStats()
	if err != nil {
		t.Fatalf("GetGlobalStats: %v", err)
	}

	if stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", stats.TotalFiles)
	}
	if stats.TotalShares != 1 {
		t.Errorf("TotalShares = %d, want 1", stats.TotalShares)
	}
	if stats.TotalDownloads != 1 {
		t.Errorf("TotalDownloads = %d, want 1", stats.TotalDownloads)
	}
	if stats.ActiveShares != 1 {
		t.Errorf("ActiveShares = %d, want 1", stats.ActiveShares)
	}
	if stats.TotalStorage != 1024 {
		t.Errorf("TotalStorage = %d, want 1024", stats.TotalStorage)
	}
}

func TestStatsService_EmptyGlobalStats(t *testing.T) {
	db := openTestDB(t)
	svc := NewStatsService(db.DB)

	stats, err := svc.GetGlobalStats()
	if err != nil {
		t.Fatalf("GetGlobalStats: %v", err)
	}

	if stats.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0", stats.TotalFiles)
	}
	if stats.TotalShares != 0 {
		t.Errorf("TotalShares = %d, want 0", stats.TotalShares)
	}
}

func TestStatsService_EmptyShareStats(t *testing.T) {
	db := openTestDB(t)
	svc := NewStatsService(db.DB)

	stats, err := svc.GetShareStats("nonexistent")
	if err != nil {
		t.Fatalf("GetShareStats: %v", err)
	}

	if stats.TotalDownloads != 0 {
		t.Errorf("TotalDownloads = %d, want 0", stats.TotalDownloads)
	}
	if stats.UniqueIPs != 0 {
		t.Errorf("UniqueIPs = %d, want 0", stats.UniqueIPs)
	}
	if len(stats.RecentDownloads) != 0 {
		t.Errorf("len(RecentDownloads) = %d, want 0", len(stats.RecentDownloads))
	}
}


