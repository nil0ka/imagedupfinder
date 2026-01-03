package match

import (
	"sort"

	"imagedupfinder/internal/models"
)

// Matcher is the interface for duplicate detection strategies
type Matcher interface {
	FindGroups(images []*models.ImageInfo) []*models.DuplicateGroup
}

// buildGroups builds DuplicateGroup slice from a group map
func buildGroups(groupMap map[int][]*models.ImageInfo) []*models.DuplicateGroup {
	var groups []*models.DuplicateGroup
	groupID := 1

	for _, imgs := range groupMap {
		if len(imgs) < 2 {
			continue
		}

		group := &models.DuplicateGroup{
			ID:     groupID,
			Images: imgs,
		}

		selectKeepAndRemove(group)
		groups = append(groups, group)
		groupID++
	}

	// Sort groups by ID for consistent output
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})

	return groups
}

// selectKeepAndRemove determines which image to keep and which to remove
func selectKeepAndRemove(group *models.DuplicateGroup) {
	if len(group.Images) == 0 {
		return
	}

	// Sort images by score (descending), then by file size (descending),
	// then by mod time (descending), then by path (ascending)
	sorted := make([]*models.ImageInfo, len(group.Images))
	copy(sorted, group.Images)

	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]

		// Primary: score (higher is better)
		if a.Score != b.Score {
			return a.Score > b.Score
		}

		// Secondary: file size (larger is better - more information)
		if a.FileSize != b.FileSize {
			return a.FileSize > b.FileSize
		}

		// Tertiary: mod time (newer is better)
		if !a.ModTime.Equal(b.ModTime) {
			return a.ModTime.After(b.ModTime)
		}

		// Fallback: path (alphabetical)
		return a.Path < b.Path
	})

	// First image is the one to keep
	group.Keep = sorted[0]

	// Rest are to be removed
	group.Remove = sorted[1:]

	// Assign group ID to all images
	for _, img := range group.Images {
		img.GroupID = group.ID
	}
}
