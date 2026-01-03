package models

import "time"

// ImageInfo holds metadata and hash information for an image
type ImageInfo struct {
	ID        int64     `json:"id"`
	Path      string    `json:"path"`
	Hash      uint64    `json:"hash"`
	FileHash  string    `json:"file_hash,omitempty"` // SHA256 hash for exact matching
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Format    string    `json:"format"`
	FileSize  int64     `json:"file_size"`
	ModTime   time.Time `json:"mod_time"`
	HasExif   bool      `json:"has_exif"`
	Score     float64   `json:"score"`
	GroupID   int       `json:"group_id,omitempty"`
}

// DuplicateGroup represents a group of similar images
type DuplicateGroup struct {
	ID     int          `json:"id"`
	Images []*ImageInfo `json:"images"`
	Keep   *ImageInfo   `json:"keep"`   // Image to keep (highest score)
	Remove []*ImageInfo `json:"remove"` // Images to remove
}

// ScanResult holds the result of a folder scan
type ScanResult struct {
	TotalScanned    int               `json:"total_scanned"`
	TotalGroups     int               `json:"total_groups"`
	TotalDuplicates int               `json:"total_duplicates"`
	Groups          []*DuplicateGroup `json:"groups"`
}

// FormatQualityMultiplier returns quality multiplier for image format
func FormatQualityMultiplier(format string) float64 {
	switch format {
	case "png", "tiff", "bmp":
		return 1.2 // Lossless formats
	case "webp":
		return 1.1 // Often lossless or high quality
	case "jpeg", "jpg":
		return 1.0 // Lossy
	case "gif":
		return 0.9 // Limited colors
	default:
		return 1.0
	}
}

// MetadataMultiplier returns quality multiplier based on metadata presence
func MetadataMultiplier(hasExif bool) float64 {
	if hasExif {
		return 1.1 // Prefer images with metadata
	}
	return 1.0
}
