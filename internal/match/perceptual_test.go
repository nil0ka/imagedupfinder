package match

import (
	"testing"
	"time"

	"imagedupfinder/internal/hash"
	"imagedupfinder/internal/models"
)

func TestBKTree_Empty(t *testing.T) {
	tree := newBKTree(hash.HammingDistance)

	results := tree.findWithinDistance(0, 10)
	if len(results) != 0 {
		t.Errorf("expected empty results for empty tree, got %d", len(results))
	}

	if tree.size() != 0 {
		t.Errorf("expected size 0, got %d", tree.size())
	}
}

func TestBKTree_SingleElement(t *testing.T) {
	tree := newBKTree(hash.HammingDistance)
	tree.insert(0b1111, 0)

	// Exact match
	results := tree.findWithinDistance(0b1111, 0)
	if len(results) != 1 || results[0] != 0 {
		t.Errorf("expected [0], got %v", results)
	}

	// Within threshold
	results = tree.findWithinDistance(0b1110, 1) // distance 1
	if len(results) != 1 || results[0] != 0 {
		t.Errorf("expected [0], got %v", results)
	}

	// Outside threshold
	results = tree.findWithinDistance(0b0000, 3) // distance 4
	if len(results) != 0 {
		t.Errorf("expected [], got %v", results)
	}
}

func TestBKTree_MultipleElements(t *testing.T) {
	tree := newBKTree(hash.HammingDistance)

	// Insert hashes with known distances
	hashes := []uint64{
		0b0000, // index 0
		0b0001, // index 1, distance 1 from 0
		0b0011, // index 2, distance 2 from 0, distance 1 from 1
		0b1111, // index 3, distance 4 from 0
		0b0000, // index 4, distance 0 from 0 (duplicate hash)
	}

	for i, h := range hashes {
		tree.insert(h, i)
	}

	if tree.size() != 5 {
		t.Errorf("expected size 5, got %d", tree.size())
	}

	// Find exact matches for 0b0000
	results := tree.findWithinDistance(0b0000, 0)
	if !containsAll(results, []int{0, 4}) {
		t.Errorf("expected [0, 4], got %v", results)
	}

	// Find within distance 1
	results = tree.findWithinDistance(0b0000, 1)
	if !containsAll(results, []int{0, 1, 4}) {
		t.Errorf("expected [0, 1, 4], got %v", results)
	}

	// Find within distance 2
	results = tree.findWithinDistance(0b0000, 2)
	if !containsAll(results, []int{0, 1, 2, 4}) {
		t.Errorf("expected [0, 1, 2, 4], got %v", results)
	}

	// Find all within distance 4
	results = tree.findWithinDistance(0b0000, 4)
	if !containsAll(results, []int{0, 1, 2, 3, 4}) {
		t.Errorf("expected [0, 1, 2, 3, 4], got %v", results)
	}
}

func TestBKTree_TriangleInequality(t *testing.T) {
	tree := newBKTree(hash.HammingDistance)

	// Insert many elements to test pruning
	for i := 0; i < 100; i++ {
		tree.insert(uint64(i), i)
	}

	// Should find only nearby elements
	results := tree.findWithinDistance(50, 2)

	// Verify all results are actually within distance
	for _, idx := range results {
		dist := hash.HammingDistance(50, uint64(idx))
		if dist > 2 {
			t.Errorf("found index %d with distance %d, expected <= 2", idx, dist)
		}
	}
}

func TestBKTree_LargeThreshold(t *testing.T) {
	tree := newBKTree(hash.HammingDistance)

	for i := 0; i < 10; i++ {
		tree.insert(uint64(i), i)
	}

	// Large threshold should return all
	results := tree.findWithinDistance(0, 64)
	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}
}

// Helper function to check if all expected values are in results
func containsAll(results []int, expected []int) bool {
	if len(results) != len(expected) {
		return false
	}
	found := make(map[int]bool)
	for _, r := range results {
		found[r] = true
	}
	for _, e := range expected {
		if !found[e] {
			return false
		}
	}
	return true
}

func TestPerceptualMatcher_Empty(t *testing.T) {
	matcher := NewPerceptualMatcher(10)
	groups := matcher.FindGroups(nil)
	if groups != nil {
		t.Errorf("expected nil for empty input, got %v", groups)
	}
}

func TestPerceptualMatcher_SingleImage(t *testing.T) {
	matcher := NewPerceptualMatcher(10)
	images := []*models.ImageInfo{{Hash: 0b1111}}
	groups := matcher.FindGroups(images)
	if groups != nil {
		t.Errorf("expected nil for single image, got %v", groups)
	}
}

func TestPerceptualMatcher_NoDuplicates(t *testing.T) {
	matcher := NewPerceptualMatcher(2)
	images := []*models.ImageInfo{
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
	images := []*models.ImageInfo{
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
	images := []*models.ImageInfo{
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
	images := []*models.ImageInfo{
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
	images := []*models.ImageInfo{
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

// Test that BK-Tree produces same results as brute force O(n²)
func TestPerceptualMatcher_EquivalenceWithBruteForce(t *testing.T) {
	// Create test images with various hashes
	images := make([]*models.ImageInfo, 50)
	for i := 0; i < 50; i++ {
		images[i] = &models.ImageInfo{
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
			if hash.HammingDistance(images[i].Hash, images[j].Hash) <= threshold {
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

// Benchmark comparison
func BenchmarkBKTree_Insert(b *testing.B) {
	tree := newBKTree(hash.HammingDistance)
	for i := 0; i < b.N; i++ {
		tree.insert(uint64(i*12345), i)
	}
}

func BenchmarkBKTree_Find(b *testing.B) {
	tree := newBKTree(hash.HammingDistance)
	for i := 0; i < 10000; i++ {
		tree.insert(uint64(i*12345), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.findWithinDistance(uint64(i*67890), 10)
	}
}

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

func generateTestImages(n int) []*models.ImageInfo {
	images := make([]*models.ImageInfo, n)
	for i := 0; i < n; i++ {
		images[i] = &models.ImageInfo{
			Path:    string(rune(i)),
			Hash:    uint64(i * 12345), // pseudo-random spread
			Score:   float64(i),
			ModTime: time.Now(),
		}
	}
	return images
}
