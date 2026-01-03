package storage

import (
	"path/filepath"
	"testing"
	"time"

	"imagedupfinder/internal/models"
)

func TestNewStorage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer store.Close()

	if store.db == nil {
		t.Error("db should not be nil")
	}
}

func TestNewStorage_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "nested", "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage failed to create directories: %v", err)
	}
	defer store.Close()
}

func TestSaveImages_AndGetAllImages(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer store.Close()

	images := []*models.ImageInfo{
		{
			Path:     "/path/to/image1.jpg",
			Hash:     12345,
			FileHash: "abc123",
			Width:    1920,
			Height:   1080,
			Format:   "jpeg",
			FileSize: 1024000,
			ModTime:  time.Now(),
			HasExif:  true,
			Score:    2073600,
			GroupID:  0,
		},
		{
			Path:     "/path/to/image2.png",
			Hash:     67890,
			FileHash: "def456",
			Width:    800,
			Height:   600,
			Format:   "png",
			FileSize: 512000,
			ModTime:  time.Now(),
			HasExif:  false,
			Score:    480000,
			GroupID:  0,
		},
	}

	if err := store.SaveImages(images); err != nil {
		t.Fatalf("SaveImages failed: %v", err)
	}

	retrieved, err := store.GetAllImages()
	if err != nil {
		t.Fatalf("GetAllImages failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Fatalf("expected 2 images, got %d", len(retrieved))
	}

	// Check first image
	img := retrieved[0]
	if img.Path != "/path/to/image1.jpg" {
		t.Errorf("path = %q, want /path/to/image1.jpg", img.Path)
	}
	if img.Hash != 12345 {
		t.Errorf("hash = %d, want 12345", img.Hash)
	}
	if img.FileHash != "abc123" {
		t.Errorf("file_hash = %q, want abc123", img.FileHash)
	}
	if img.Width != 1920 || img.Height != 1080 {
		t.Errorf("dimensions = %dx%d, want 1920x1080", img.Width, img.Height)
	}
	if !img.HasExif {
		t.Error("HasExif should be true")
	}
}

func TestSaveImages_Upsert(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer store.Close()

	// Save initial image
	images := []*models.ImageInfo{{
		Path:     "/path/to/image.jpg",
		Hash:     12345,
		Width:    100,
		Height:   100,
		Format:   "jpeg",
		FileSize: 1000,
		ModTime:  time.Now(),
		Score:    10000,
	}}

	if err := store.SaveImages(images); err != nil {
		t.Fatalf("first SaveImages failed: %v", err)
	}

	// Update with different values
	images[0].Width = 200
	images[0].Height = 200
	images[0].Score = 40000

	if err := store.SaveImages(images); err != nil {
		t.Fatalf("second SaveImages failed: %v", err)
	}

	retrieved, err := store.GetAllImages()
	if err != nil {
		t.Fatalf("GetAllImages failed: %v", err)
	}

	if len(retrieved) != 1 {
		t.Fatalf("expected 1 image after upsert, got %d", len(retrieved))
	}

	if retrieved[0].Width != 200 {
		t.Errorf("width after upsert = %d, want 200", retrieved[0].Width)
	}
}

func TestUpdateGroups(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer store.Close()

	// Save images first
	images := []*models.ImageInfo{
		{Path: "/img1.jpg", Hash: 1, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 10000},
		{Path: "/img2.jpg", Hash: 1, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 9000},
		{Path: "/img3.jpg", Hash: 2, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 8000},
	}

	if err := store.SaveImages(images); err != nil {
		t.Fatalf("SaveImages failed: %v", err)
	}

	// Create groups
	groups := []*models.DuplicateGroup{
		{
			ID:     1,
			Images: []*models.ImageInfo{images[0], images[1]},
			Keep:   images[0],
			Remove: []*models.ImageInfo{images[1]},
		},
	}

	if err := store.UpdateGroups(groups); err != nil {
		t.Fatalf("UpdateGroups failed: %v", err)
	}

	// Check group assignments
	groupImages, err := store.GetImagesByGroupID(1)
	if err != nil {
		t.Fatalf("GetImagesByGroupID failed: %v", err)
	}

	if len(groupImages) != 2 {
		t.Errorf("expected 2 images in group 1, got %d", len(groupImages))
	}
}

func TestGetDuplicateGroups(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer store.Close()

	// Save images with group IDs
	images := []*models.ImageInfo{
		{Path: "/img1.jpg", Hash: 1, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 10000, GroupID: 1},
		{Path: "/img2.jpg", Hash: 1, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 9000, GroupID: 1},
		{Path: "/img3.jpg", Hash: 2, Width: 200, Height: 200, Format: "png", FileSize: 2000, ModTime: time.Now(), Score: 48000, GroupID: 2},
		{Path: "/img4.jpg", Hash: 2, Width: 200, Height: 200, Format: "png", FileSize: 2000, ModTime: time.Now(), Score: 40000, GroupID: 2},
		{Path: "/img5.jpg", Hash: 3, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 10000, GroupID: 0}, // No group
	}

	if err := store.SaveImages(images); err != nil {
		t.Fatalf("SaveImages failed: %v", err)
	}

	groups, err := store.GetDuplicateGroups()
	if err != nil {
		t.Fatalf("GetDuplicateGroups failed: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	// Check first group
	if len(groups[0].Images) != 2 {
		t.Errorf("group 1 should have 2 images, got %d", len(groups[0].Images))
	}
	if groups[0].Keep == nil {
		t.Error("group 1 Keep should not be nil")
	}
	if len(groups[0].Remove) != 1 {
		t.Errorf("group 1 should have 1 remove, got %d", len(groups[0].Remove))
	}
}

func TestDeleteImage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer store.Close()

	images := []*models.ImageInfo{
		{Path: "/img1.jpg", Hash: 1, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 10000},
		{Path: "/img2.jpg", Hash: 2, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 10000},
	}

	if err := store.SaveImages(images); err != nil {
		t.Fatalf("SaveImages failed: %v", err)
	}

	if err := store.DeleteImage("/img1.jpg"); err != nil {
		t.Fatalf("DeleteImage failed: %v", err)
	}

	remaining, err := store.GetAllImages()
	if err != nil {
		t.Fatalf("GetAllImages failed: %v", err)
	}

	if len(remaining) != 1 {
		t.Errorf("expected 1 image after delete, got %d", len(remaining))
	}
	if remaining[0].Path != "/img2.jpg" {
		t.Errorf("wrong image remained: %s", remaining[0].Path)
	}
}

func TestRecordScan(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer store.Close()

	err = store.RecordScan("/path/to/folder", 100, 10, 25)
	if err != nil {
		t.Fatalf("RecordScan failed: %v", err)
	}

	// Verify by querying directly
	var folder string
	var total, groups, dups int
	err = store.db.QueryRow("SELECT folder, total_images, total_groups, total_duplicates FROM scan_history LIMIT 1").Scan(&folder, &total, &groups, &dups)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if folder != "/path/to/folder" {
		t.Errorf("folder = %q, want /path/to/folder", folder)
	}
	if total != 100 || groups != 10 || dups != 25 {
		t.Errorf("stats = (%d, %d, %d), want (100, 10, 25)", total, groups, dups)
	}
}

func TestGetGroupCount(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer store.Close()

	// Initially no groups
	count, err := store.GetGroupCount()
	if err != nil {
		t.Fatalf("GetGroupCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("initial count = %d, want 0", count)
	}

	// Add images with groups
	images := []*models.ImageInfo{
		{Path: "/img1.jpg", Hash: 1, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 10000, GroupID: 1},
		{Path: "/img2.jpg", Hash: 1, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 9000, GroupID: 1},
		{Path: "/img3.jpg", Hash: 2, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 10000, GroupID: 2},
		{Path: "/img4.jpg", Hash: 2, Width: 100, Height: 100, Format: "jpeg", FileSize: 1000, ModTime: time.Now(), Score: 9000, GroupID: 2},
	}

	if err := store.SaveImages(images); err != nil {
		t.Fatalf("SaveImages failed: %v", err)
	}

	count, err = store.GetGroupCount()
	if err != nil {
		t.Fatalf("GetGroupCount failed: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	// Check schema version
	version := store.getSchemaVersion()
	if version != schemaVersion {
		t.Errorf("schema version = %d, want %d", version, schemaVersion)
	}

	// Check file_hash column exists
	if !store.columnExists("images", "file_hash") {
		t.Error("file_hash column should exist after migrations")
	}

	store.Close()

	// Reopen - should not fail
	store2, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("second NewStorage failed: %v", err)
	}
	defer store2.Close()

	version2 := store2.getSchemaVersion()
	if version2 != schemaVersion {
		t.Errorf("schema version after reopen = %d, want %d", version2, schemaVersion)
	}
}
