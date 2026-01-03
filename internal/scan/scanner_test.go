package scan

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewScanner_Defaults(t *testing.T) {
	s := NewScanner()

	if s.workers != 8 {
		t.Errorf("default workers = %d, want 8", s.workers)
	}
	if s.timeout != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", s.timeout)
	}
	if s.progressFn != nil {
		t.Error("default progressFn should be nil")
	}
}

func TestNewScanner_WithWorkers(t *testing.T) {
	s := NewScanner(WithWorkers(4))
	if s.workers != 4 {
		t.Errorf("workers = %d, want 4", s.workers)
	}

	// Zero workers should not change default
	s = NewScanner(WithWorkers(0))
	if s.workers != 8 {
		t.Errorf("workers with 0 = %d, want 8", s.workers)
	}

	// Negative workers should not change default
	s = NewScanner(WithWorkers(-1))
	if s.workers != 8 {
		t.Errorf("workers with -1 = %d, want 8", s.workers)
	}
}

func TestNewScanner_WithTimeout(t *testing.T) {
	s := NewScanner(WithTimeout(5 * time.Second))
	if s.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", s.timeout)
	}
}

func TestNewScanner_WithProgress(t *testing.T) {
	called := false
	fn := func(scanned, total int, current string) {
		called = true
	}

	s := NewScanner(WithProgress(fn))
	if s.progressFn == nil {
		t.Error("progressFn should not be nil")
	}

	s.progressFn(1, 10, "test.jpg")
	if !called {
		t.Error("progressFn was not called")
	}
}

func TestNewScanner_MultipleOptions(t *testing.T) {
	s := NewScanner(
		WithWorkers(16),
		WithTimeout(10*time.Second),
		WithProgress(func(_, _ int, _ string) {}),
	)

	if s.workers != 16 {
		t.Errorf("workers = %d, want 16", s.workers)
	}
	if s.timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", s.timeout)
	}
	if s.progressFn == nil {
		t.Error("progressFn should not be nil")
	}
}

func TestScanFolder_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	s := NewScanner()
	images, err := s.ScanFolder(tmpDir)

	if err != nil {
		t.Fatalf("ScanFolder failed: %v", err)
	}
	if images != nil {
		t.Errorf("expected nil for empty directory, got %d images", len(images))
	}
}

func TestScanFolder_NoImages(t *testing.T) {
	tmpDir := t.TempDir()

	// Create non-image files
	files := []string{"test.txt", "doc.pdf", "script.sh"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	s := NewScanner()
	images, err := s.ScanFolder(tmpDir)

	if err != nil {
		t.Fatalf("ScanFolder failed: %v", err)
	}
	if images != nil {
		t.Errorf("expected nil for non-image files, got %d images", len(images))
	}
}

func TestScanFolder_WithImages(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal PNG (1x1 pixel)
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
		0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54,
		0x08, 0xD7, 0x63, 0xF8, 0xFF, 0xFF, 0x3F, 0x00,
		0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC, 0x59, 0xE7,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44,
		0xAE, 0x42, 0x60, 0x82,
	}

	imageFiles := []string{"img1.png", "img2.png"}
	for _, f := range imageFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, f), pngData, 0644); err != nil {
			t.Fatalf("failed to create image: %v", err)
		}
	}

	s := NewScanner(WithWorkers(2))
	images, err := s.ScanFolder(tmpDir)

	if err != nil {
		t.Fatalf("ScanFolder failed: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images, got %d", len(images))
	}
}

func TestScanFolder_Recursive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectory structure
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
		0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54,
		0x08, 0xD7, 0x63, 0xF8, 0xFF, 0xFF, 0x3F, 0x00,
		0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC, 0x59, 0xE7,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44,
		0xAE, 0x42, 0x60, 0x82,
	}

	// Create image in root and subdir
	if err := os.WriteFile(filepath.Join(tmpDir, "root.png"), pngData, 0644); err != nil {
		t.Fatalf("failed to create root image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "sub.png"), pngData, 0644); err != nil {
		t.Fatalf("failed to create sub image: %v", err)
	}

	s := NewScanner()
	images, err := s.ScanFolder(tmpDir)

	if err != nil {
		t.Fatalf("ScanFolder failed: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images (recursive), got %d", len(images))
	}
}

func TestScanFolder_ProgressCallback(t *testing.T) {
	tmpDir := t.TempDir()

	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
		0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54,
		0x08, 0xD7, 0x63, 0xF8, 0xFF, 0xFF, 0x3F, 0x00,
		0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC, 0x59, 0xE7,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44,
		0xAE, 0x42, 0x60, 0x82,
	}

	for i := 0; i < 3; i++ {
		if err := os.WriteFile(filepath.Join(tmpDir, filepath.Base(t.Name())+string(rune('a'+i))+".png"), pngData, 0644); err != nil {
			t.Fatalf("failed to create image: %v", err)
		}
	}

	var callCount int64
	s := NewScanner(
		WithWorkers(1),
		WithProgress(func(scanned, total int, current string) {
			atomic.AddInt64(&callCount, 1)
			if total != 3 {
				t.Errorf("total = %d, want 3", total)
			}
		}),
	)

	_, err := s.ScanFolder(tmpDir)
	if err != nil {
		t.Fatalf("ScanFolder failed: %v", err)
	}

	if callCount != 3 {
		t.Errorf("progress called %d times, want 3", callCount)
	}
}

func TestScanFolders_Multiple(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
		0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54,
		0x08, 0xD7, 0x63, 0xF8, 0xFF, 0xFF, 0x3F, 0x00,
		0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC, 0x59, 0xE7,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44,
		0xAE, 0x42, 0x60, 0x82,
	}

	if err := os.WriteFile(filepath.Join(tmpDir1, "img1.png"), pngData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir2, "img2.png"), pngData, 0644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner()
	images, err := s.ScanFolders([]string{tmpDir1, tmpDir2})

	if err != nil {
		t.Fatalf("ScanFolders failed: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images from 2 folders, got %d", len(images))
	}
}
