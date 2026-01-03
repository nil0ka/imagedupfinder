package match

import "imagedupfinder/internal/models"

// ExactMatcher finds groups of images with identical file hashes
type ExactMatcher struct{}

// NewExactMatcher creates a new ExactMatcher
func NewExactMatcher() *ExactMatcher {
	return &ExactMatcher{}
}

// FindGroups finds groups of images with identical file hashes
func (m *ExactMatcher) FindGroups(images []*models.ImageInfo) []*models.DuplicateGroup {
	if len(images) < 2 {
		return nil
	}

	// Group by file hash
	hashMap := make(map[string][]*models.ImageInfo)
	for _, img := range images {
		if img.FileHash != "" {
			hashMap[img.FileHash] = append(hashMap[img.FileHash], img)
		}
	}

	// Convert to group map format
	groupMap := make(map[int][]*models.ImageInfo)
	idx := 0
	for _, imgs := range hashMap {
		groupMap[idx] = imgs
		idx++
	}

	return buildGroups(groupMap)
}
