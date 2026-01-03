package internal

import (
	"sort"
	"testing"
	"time"
)

func TestPerceptualMatcher_Empty(t *testing.T) {
	matcher := NewPerceptualMatcher(10)
	groups := matcher.FindGroups(nil)
	if groups != nil {
		t.Errorf("expected nil for empty input, got %v", groups)
	}
}

func TestPerceptualMatcher_SingleImage(t *testing.T) {
	matcher := NewPerceptualMatcher(10)
	images := []*ImageInfo{{Hash: 0b1111}}
	groups := matcher.FindGroups(images)
	if groups != nil {
		t.Errorf("expected nil for single image, got %v", groups)
	}
}

func TestPerceptualMatcher_NoDuplicates(t *testing.T) {
	matcher := NewPerceptualMatcher(2)
	images := []*ImageInfo{
		{Path: "a.jpg", Hash: 0b0000000000},
		{Path: "b.jpg", Hash: 0b1111111111}, // distance > 2
	}
	groups := matcher.FindGroups(images)
	if len(groups) != 0 {
		t.Errorf("expected no groups for distant images, got %d", len(groups))
	}
}

func TestPerceptualMatcher_ExactDuplicates(t *testing.T) {
	matcher := NewPerceptualMatcher(0)
	images := []*ImageInfo{
		{Path: "a.jpg", Hash: 0b1111, Score: 1.0},
		{Path: "b.jpg", Hash: 0b1111, Score: 2.0}, // same hash
		{Path: "c.jpg", Hash: 0b0000, Score: 1.0}, // different hash
	}
	groups := matcher.FindGroups(images)
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Images) != 2 {
		t.Errorf("expected 2 images in group, got %d", len(groups[0].Images))
	}
}

func TestPerceptualMatcher_SimilarImages(t *testing.T) {
	matcher := NewPerceptualMatcher(2)
	images := []*ImageInfo{
		{Path: "a.jpg", Hash: 0b00000000, Score: 1.0},
		{Path: "b.jpg", Hash: 0b00000001, Score: 2.0}, // distance 1 from a
		{Path: "c.jpg", Hash: 0b00000011, Score: 1.5}, // distance 2 from a, 1 from b
		{Path: "d.jpg", Hash: 0b11111111, Score: 1.0}, // distance 6 from c (outside threshold)
	}
	groups := matcher.FindGroups(images)
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Images) != 3 {
		t.Errorf("expected 3 images in group (a, b, c), got %d", len(groups[0].Images))
	}
}

func TestPerceptualMatcher_MultipleGroups(t *testing.T) {
	matcher := NewPerceptualMatcher(1)
	images := []*ImageInfo{
		{Path: "a.jpg", Hash: 0x0000000000000000, Score: 1.0},
		{Path: "b.jpg", Hash: 0x0000000000000001, Score: 2.0}, // group 1
		{Path: "c.jpg", Hash: 0xFFFFFFFFFFFFFFFF, Score: 1.0},
		{Path: "d.jpg", Hash: 0xFFFFFFFFFFFFFFFE, Score: 2.0}, // group 2
	}
	groups := matcher.FindGroups(images)
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestPerceptualMatcher_KeepHighestScore(t *testing.T) {
	matcher := NewPerceptualMatcher(10)
	images := []*ImageInfo{
		{Path: "low.jpg", Hash: 0b0000, Score: 1.0},
		{Path: "high.jpg", Hash: 0b0001, Score: 10.0},
		{Path: "mid.jpg", Hash: 0b0010, Score: 5.0},
	}
	groups := matcher.FindGroups(images)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Keep.Path != "high.jpg" {
		t.Errorf("expected to keep high.jpg, got %s", groups[0].Keep.Path)
	}
	if len(groups[0].Remove) != 2 {
		t.Errorf("expected 2 images to remove, got %d", len(groups[0].Remove))
	}
}

func TestExactMatcher_Empty(t *testing.T) {
	matcher := NewExactMatcher()
	groups := matcher.FindGroups(nil)
	if groups != nil {
		t.Errorf("expected nil for empty input, got %v", groups)
	}
}

func TestExactMatcher_NoDuplicates(t *testing.T) {
	matcher := NewExactMatcher()
	images := []*ImageInfo{
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
	images := []*ImageInfo{
		{Path: "a.jpg", FileHash: "abc123", Score: 1.0},
		{Path: "b.jpg", FileHash: "abc123", Score: 2.0}, // same hash
		{Path: "c.jpg", FileHash: "def456", Score: 1.0},
	}
	groups := matcher.FindGroups(images)
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
}

// Test that BK-Tree produces same results as brute force O(n²)
func TestPerceptualMatcher_EquivalenceWithBruteForce(t *testing.T) {
	// Create test images with various hashes
	images := make([]*ImageInfo, 50)
	for i := 0; i < 50; i++ {
		images[i] = &ImageInfo{
			Path:  string(rune('a' + i)),
			Hash:  uint64(i * 7), // spread out hashes
			Score: float64(i),
		}
	}

	threshold := 5

	// Get groups from BK-Tree implementation
	matcher := NewPerceptualMatcher(threshold)
	bkGroups := matcher.FindGroups(images)

	// Compute expected groups with brute force
	uf := newUnionFind(len(images))
	for i := 0; i < len(images); i++ {
		for j := i + 1; j < len(images); j++ {
			if HammingDistance(images[i].Hash, images[j].Hash) <= threshold {
				uf.union(i, j)
			}
		}
	}
	groupMap := make(map[int][]int)
	for i := range images {
		root := uf.find(i)
		groupMap[root] = append(groupMap[root], i)
	}
	expectedGroupCount := 0
	for _, indices := range groupMap {
		if len(indices) >= 2 {
			expectedGroupCount++
		}
	}

	if len(bkGroups) != expectedGroupCount {
		t.Errorf("BK-Tree found %d groups, brute force found %d", len(bkGroups), expectedGroupCount)
	}
}

// Benchmark comparison
func BenchmarkPerceptualMatcher_1000(b *testing.B) {
	images := generateTestImages(1000)
	matcher := NewPerceptualMatcher(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.FindGroups(images)
	}
}

func BenchmarkPerceptualMatcher_5000(b *testing.B) {
	images := generateTestImages(5000)
	matcher := NewPerceptualMatcher(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.FindGroups(images)
	}
}

func generateTestImages(n int) []*ImageInfo {
	images := make([]*ImageInfo, n)
	for i := 0; i < n; i++ {
		images[i] = &ImageInfo{
			Path:    string(rune(i)),
			Hash:    uint64(i * 12345), // pseudo-random spread
			Score:   float64(i),
			ModTime: time.Now(),
		}
	}
	return images
}

func TestUnionFind(t *testing.T) {
	uf := newUnionFind(5)

	// Initially all separate
	for i := 0; i < 5; i++ {
		if uf.find(i) != i {
			t.Errorf("expected %d to be its own root", i)
		}
	}

	// Union 0 and 1
	uf.union(0, 1)
	if uf.find(0) != uf.find(1) {
		t.Error("expected 0 and 1 to be in same group")
	}

	// Union 2 and 3
	uf.union(2, 3)
	if uf.find(2) != uf.find(3) {
		t.Error("expected 2 and 3 to be in same group")
	}

	// 4 should still be separate
	if uf.find(4) == uf.find(0) || uf.find(4) == uf.find(2) {
		t.Error("expected 4 to be separate")
	}

	// Union the two groups
	uf.union(1, 3)
	if uf.find(0) != uf.find(2) {
		t.Error("expected all of 0,1,2,3 to be in same group")
	}
}

func TestSelectKeepAndRemove(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		images       []*ImageInfo
		expectedKeep string
	}{
		{
			name: "keep highest score",
			images: []*ImageInfo{
				{Path: "low.jpg", Score: 1.0, FileSize: 100, ModTime: now},
				{Path: "high.jpg", Score: 10.0, FileSize: 100, ModTime: now},
			},
			expectedKeep: "high.jpg",
		},
		{
			name: "tie score, keep larger file",
			images: []*ImageInfo{
				{Path: "small.jpg", Score: 5.0, FileSize: 100, ModTime: now},
				{Path: "large.jpg", Score: 5.0, FileSize: 1000, ModTime: now},
			},
			expectedKeep: "large.jpg",
		},
		{
			name: "tie score and size, keep newer",
			images: []*ImageInfo{
				{Path: "old.jpg", Score: 5.0, FileSize: 100, ModTime: now.Add(-time.Hour)},
				{Path: "new.jpg", Score: 5.0, FileSize: 100, ModTime: now},
			},
			expectedKeep: "new.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &DuplicateGroup{ID: 1, Images: tt.images}
			selectKeepAndRemove(group)
			if group.Keep.Path != tt.expectedKeep {
				t.Errorf("expected to keep %s, got %s", tt.expectedKeep, group.Keep.Path)
			}
		})
	}
}

func TestBuildGroups(t *testing.T) {
	images := []*ImageInfo{
		{Path: "a.jpg", Score: 1.0},
		{Path: "b.jpg", Score: 2.0},
		{Path: "c.jpg", Score: 3.0},
	}

	groupMap := map[int][]*ImageInfo{
		0: {images[0], images[1]}, // group of 2
		1: {images[2]},            // single (should be excluded)
	}

	groups := buildGroups(groupMap)

	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}

	// Verify group is properly sorted by ID
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})

	if groups[0].Keep.Path != "b.jpg" {
		t.Errorf("expected b.jpg to be kept (higher score), got %s", groups[0].Keep.Path)
	}
}
