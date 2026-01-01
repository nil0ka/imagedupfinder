package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corona10/goimagehash"
	"github.com/rwcarlsen/goexif/exif"
	_ "golang.org/x/image/webp"
)

// Hasher computes perceptual hashes for images
type Hasher struct{}

// NewHasher creates a new Hasher
func NewHasher() *Hasher {
	return &Hasher{}
}

// HashImage computes the perceptual hash and extracts metadata for an image
func (h *Hasher) HashImage(path string) (*ImageInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check for EXIF data (before reading image, as Decode consumes the reader)
	hasExif := checkExif(path)

	// Decode image
	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Compute perceptual hash
	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	info := &ImageInfo{
		Path:     path,
		Hash:     hash.GetHash(),
		Width:    width,
		Height:   height,
		Format:   strings.ToLower(format),
		FileSize: stat.Size(),
		ModTime:  stat.ModTime(),
		HasExif:  hasExif,
	}

	// Calculate score
	info.Score = h.CalculateScore(info)

	return info, nil
}

// checkExif checks if an image file contains EXIF data
func checkExif(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	_, err = exif.Decode(file)
	return err == nil
}

// CalculateScore computes the quality score for an image
func (h *Hasher) CalculateScore(info *ImageInfo) float64 {
	// Base score: resolution (width * height)
	resolution := float64(info.Width * info.Height)

	// Apply format quality multiplier
	formatMultiplier := FormatQualityMultiplier(info.Format)

	// Apply metadata multiplier (prefer images with EXIF)
	metadataMultiplier := MetadataMultiplier(info.HasExif)

	return resolution * formatMultiplier * metadataMultiplier
}

// ComputeFileHash computes the SHA256 hash of a file
func ComputeFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// IsSupportedImage checks if a file is a supported image format
func IsSupportedImage(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tiff", ".tif":
		return true
	default:
		return false
	}
}

// HammingDistance calculates the Hamming distance between two hashes
func HammingDistance(hash1, hash2 uint64) int {
	xor := hash1 ^ hash2
	count := 0
	for xor != 0 {
		count++
		xor &= xor - 1
	}
	return count
}

// HashImageWithTimeout hashes an image with a timeout
func (h *Hasher) HashImageWithTimeout(path string, timeout time.Duration) (*ImageInfo, error) {
	done := make(chan struct{})
	var info *ImageInfo
	var err error

	go func() {
		info, err = h.HashImage(path)
		close(done)
	}()

	select {
	case <-done:
		return info, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout hashing image: %s", path)
	}
}
