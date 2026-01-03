package match

import (
	"testing"

	"imagedupfinder/internal/models"
)

func TestExactMatcher_Empty(t *testing.T) {
	matcher := NewExactMatcher()
	groups := matcher.FindGroups(nil)
	if groups != nil {
		t.Errorf("expected nil for empty input, got %v", groups)
	}
}

func TestExactMatcher_NoDuplicates(t *testing.T) {
	matcher := NewExactMatcher()
	images := []*models.ImageInfo{
		{Path: "a.jpg", FileHash: "abc123"},
		{Path: "b.jpg", FileHash: "def456"},
	}
	groups := matcher.FindGroups(images)
	if len(groups) != 0 {
		t.Errorf("expected no groups, got %d", len(groups))
	}
}

func TestExactMatcher_Duplicates(t *testing.T) {
	matcher := NewExactMatcher()
	images := []*models.ImageInfo{
		{Path: "a.jpg", FileHash: "abc123", Score: 1.0},
		{Path: "b.jpg", FileHash: "abc123", Score: 2.0}, // same hash
		{Path: "c.jpg", FileHash: "def456", Score: 1.0},
	}
	groups := matcher.FindGroups(images)
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
}
