package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math/bits"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corona10/goimagehash"
	"github.com/rwcarlsen/goexif/exif"
	_ "golang.org/x/image/webp"

	"imagedupfinder/internal/models"
)

// Hasher computes perceptual hashes for images
type Hasher struct{}

// NewHasher creates a new Hasher
func NewHasher() *Hasher {
	return &Hasher{}
}

// HashImage computes the perceptual hash and extracts metadata for an image
func (h *Hasher) HashImage(path string) (*models.ImageInfo, error) {
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

	// Check for EXIF data first (Decode consumes the reader), then rewind so
	// the same open file handle can be reused for decoding. This avoids a
	// second os.Open + read of the file just to inspect EXIF.
	_, exifErr := exif.Decode(file)
	hasExif := exifErr == nil
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to rewind file: %w", err)
	}

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

	info := &models.ImageInfo{
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

// CalculateScore computes the quality score for an image
func (h *Hasher) CalculateScore(info *models.ImageInfo) float64 {
	// Base score: resolution (width * height)
	resolution := float64(info.Width * info.Height)

	// Apply format quality multiplier
	formatMultiplier := models.FormatQualityMultiplier(info.Format)

	// Apply metadata multiplier (prefer images with EXIF)
	metadataMultiplier := models.MetadataMultiplier(info.HasExif)

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

// HammingDistance calculates the Hamming distance between two hashes.
// Uses bits.OnesCount64, which compiles to a single POPCNT instruction on
// supported CPUs. This is the hottest function in perceptual matching
// (called for every BK-tree node visited).
func HammingDistance(hash1, hash2 uint64) int {
	return bits.OnesCount64(hash1 ^ hash2)
}

// HashImageWithTimeout hashes an image with a timeout.
//
// Note: image.Decode is not cancellable, so on timeout the worker goroutine
// runs to completion in the background. Results are passed over a buffered
// channel so that late completion neither blocks the goroutine nor races with
// the caller on shared variables.
func (h *Hasher) HashImageWithTimeout(path string, timeout time.Duration) (*models.ImageInfo, error) {
	type result struct {
		info *models.ImageInfo
		err  error
	}
	done := make(chan result, 1) // buffered: goroutine never blocks even after timeout

	go func() {
		info, err := h.HashImage(path)
		done <- result{info, err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case r := <-done:
		return r.info, r.err
	case <-timer.C:
		return nil, fmt.Errorf("timeout hashing image: %s", path)
	}
}
