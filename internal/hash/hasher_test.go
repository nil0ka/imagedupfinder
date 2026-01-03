package hash

import (
	"os"
	"path/filepath"
	"testing"

	"imagedupfinder/internal/models"
)

func TestHammingDistance(t *testing.T) {
	tests := []struct {
		name     string
		hash1    uint64
		hash2    uint64
		expected int
	}{
		{"identical", 0, 0, 0},
		{"one bit", 1, 0, 1},
		{"two bits", 3, 0, 2},
		{"all bits", 0xFFFFFFFFFFFFFFFF, 0, 64},
		{"half bits", 0xAAAAAAAAAAAAAAAA, 0x5555555555555555, 64},
		{"similar", 0x8000000000000000, 0x8000000000000001, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HammingDistance(tt.hash1, tt.hash2)
			if got != tt.expected {
				t.Errorf("HammingDistance(%x, %x) = %d, want %d", tt.hash1, tt.hash2, got, tt.expected)
			}
		})
	}
}

func TestIsSupportedImage(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"photo.jpg", true},
		{"photo.jpeg", true},
		{"photo.JPG", true},
		{"photo.png", true},
		{"photo.PNG", true},
		{"photo.gif", true},
		{"photo.webp", true},
		{"photo.bmp", true},
		{"photo.tiff", true},
		{"photo.tif", true},
		{"document.pdf", false},
		{"video.mp4", false},
		{"text.txt", false},
		{"noextension", false},
		{"/path/to/photo.jpg", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsSupportedImage(tt.path)
			if got != tt.expected {
				t.Errorf("IsSupportedImage(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestComputeFileHash(t *testing.T) {
	// Create temp file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hash, err := ComputeFileHash(testFile)
	if err != nil {
		t.Fatalf("ComputeFileHash failed: %v", err)
	}

	// SHA256 of "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if hash != expected {
		t.Errorf("ComputeFileHash = %q, want %q", hash, expected)
	}
}

func TestComputeFileHash_NonExistent(t *testing.T) {
	_, err := ComputeFileHash("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestCalculateScore(t *testing.T) {
	h := NewHasher()

	tests := []struct {
		name     string
		info     *models.ImageInfo
		expected float64
	}{
		{
			name: "basic jpeg",
			info: &models.ImageInfo{
				Width:   1920,
				Height:  1080,
				Format:  "jpeg",
				HasExif: false,
			},
			expected: float64(1920*1080) * 1.0 * 1.0,
		},
		{
			name: "png with exif",
			info: &models.ImageInfo{
				Width:   1920,
				Height:  1080,
				Format:  "png",
				HasExif: true,
			},
			expected: float64(1920*1080) * 1.2 * 1.1,
		},
		{
			name: "gif",
			info: &models.ImageInfo{
				Width:   640,
				Height:  480,
				Format:  "gif",
				HasExif: false,
			},
			expected: float64(640*480) * 0.9 * 1.0,
		},
		{
			name: "webp with exif",
			info: &models.ImageInfo{
				Width:   800,
				Height:  600,
				Format:  "webp",
				HasExif: true,
			},
			expected: float64(800*600) * 1.1 * 1.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.CalculateScore(tt.info)
			if got != tt.expected {
				t.Errorf("CalculateScore() = %f, want %f", got, tt.expected)
			}
		})
	}
}

func TestHasher_SameImage_IdenticalHash(t *testing.T) {
	// Create a simple test image
	tmpDir := t.TempDir()
	testImage := filepath.Join(tmpDir, "test.png")

	// Create minimal PNG (1x1 red pixel)
	// PNG signature + IHDR + IDAT + IEND
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR length + type
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE, // bit depth, color type, etc + CRC
		0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54, // IDAT length + type
		0x08, 0xD7, 0x63, 0xF8, 0xFF, 0xFF, 0x3F, 0x00, // compressed data
		0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC, 0x59, 0xE7, // CRC
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, // IEND length + type
		0xAE, 0x42, 0x60, 0x82, // CRC
	}

	if err := os.WriteFile(testImage, pngData, 0644); err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}

	h := NewHasher()

	info1, err := h.HashImage(testImage)
	if err != nil {
		t.Fatalf("first HashImage failed: %v", err)
	}

	info2, err := h.HashImage(testImage)
	if err != nil {
		t.Fatalf("second HashImage failed: %v", err)
	}

	if info1.Hash != info2.Hash {
		t.Errorf("same image should have identical hash: %d != %d", info1.Hash, info2.Hash)
	}
}
