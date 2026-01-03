package match

import (
	"testing"
	"time"

	"imagedupfinder/internal/models"
)

func TestSelectKeepAndRemove(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		images       []*models.ImageInfo
		expectedKeep string
	}{
		{
			name: "keep highest score",
			images: []*models.ImageInfo{
				{Path: "low.jpg", Score: 1.0, FileSize: 100, ModTime: now},
				{Path: "high.jpg", Score: 10.0, FileSize: 100, ModTime: now},
			},
			expectedKeep: "high.jpg",
		},
		{
			name: "tie score, keep larger file",
			images: []*models.ImageInfo{
				{Path: "small.jpg", Score: 5.0, FileSize: 100, ModTime: now},
				{Path: "large.jpg", Score: 5.0, FileSize: 1000, ModTime: now},
			},
			expectedKeep: "large.jpg",
		},
		{
			name: "tie score and size, keep newer",
			images: []*models.ImageInfo{
				{Path: "old.jpg", Score: 5.0, FileSize: 100, ModTime: now.Add(-time.Hour)},
				{Path: "new.jpg", Score: 5.0, FileSize: 100, ModTime: now},
			},
			expectedKeep: "new.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &models.DuplicateGroup{ID: 1, Images: tt.images}
			selectKeepAndRemove(group)
			if group.Keep.Path != tt.expectedKeep {
				t.Errorf("expected to keep %s, got %s", tt.expectedKeep, group.Keep.Path)
			}
		})
	}
}

func TestBuildGroups(t *testing.T) {
	images := []*models.ImageInfo{
		{Path: "a.jpg", Score: 1.0},
		{Path: "b.jpg", Score: 2.0},
		{Path: "c.jpg", Score: 3.0},
	}

	groupMap := map[int][]*models.ImageInfo{
		0: {images[0], images[1]}, // group of 2
		1: {images[2]},            // single (should be excluded)
	}

	groups := buildGroups(groupMap)

	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}

	if groups[0].Keep.Path != "b.jpg" {
		t.Errorf("expected b.jpg to be kept (higher score), got %s", groups[0].Keep.Path)
	}
}
