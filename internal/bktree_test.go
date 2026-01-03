package internal

import (
	"testing"
)

func TestBKTree_Empty(t *testing.T) {
	tree := NewBKTree(HammingDistance)

	results := tree.FindWithinDistance(0, 10)
	if len(results) != 0 {
		t.Errorf("expected empty results for empty tree, got %d", len(results))
	}

	if tree.Size() != 0 {
		t.Errorf("expected size 0, got %d", tree.Size())
	}
}

func TestBKTree_SingleElement(t *testing.T) {
	tree := NewBKTree(HammingDistance)
	tree.Insert(0b1111, 0)

	// Exact match
	results := tree.FindWithinDistance(0b1111, 0)
	if len(results) != 1 || results[0] != 0 {
		t.Errorf("expected [0], got %v", results)
	}

	// Within threshold
	results = tree.FindWithinDistance(0b1110, 1) // distance 1
	if len(results) != 1 || results[0] != 0 {
		t.Errorf("expected [0], got %v", results)
	}

	// Outside threshold
	results = tree.FindWithinDistance(0b0000, 3) // distance 4
	if len(results) != 0 {
		t.Errorf("expected [], got %v", results)
	}
}

func TestBKTree_MultipleElements(t *testing.T) {
	tree := NewBKTree(HammingDistance)

	// Insert hashes with known distances
	hashes := []uint64{
		0b0000, // index 0
		0b0001, // index 1, distance 1 from 0
		0b0011, // index 2, distance 2 from 0, distance 1 from 1
		0b1111, // index 3, distance 4 from 0
		0b0000, // index 4, distance 0 from 0 (duplicate hash)
	}

	for i, h := range hashes {
		tree.Insert(h, i)
	}

	if tree.Size() != 5 {
		t.Errorf("expected size 5, got %d", tree.Size())
	}

	// Find exact matches for 0b0000
	results := tree.FindWithinDistance(0b0000, 0)
	if !containsAll(results, []int{0, 4}) {
		t.Errorf("expected [0, 4], got %v", results)
	}

	// Find within distance 1
	results = tree.FindWithinDistance(0b0000, 1)
	if !containsAll(results, []int{0, 1, 4}) {
		t.Errorf("expected [0, 1, 4], got %v", results)
	}

	// Find within distance 2
	results = tree.FindWithinDistance(0b0000, 2)
	if !containsAll(results, []int{0, 1, 2, 4}) {
		t.Errorf("expected [0, 1, 2, 4], got %v", results)
	}

	// Find all within distance 4
	results = tree.FindWithinDistance(0b0000, 4)
	if !containsAll(results, []int{0, 1, 2, 3, 4}) {
		t.Errorf("expected [0, 1, 2, 3, 4], got %v", results)
	}
}

func TestBKTree_TriangleInequality(t *testing.T) {
	tree := NewBKTree(HammingDistance)

	// Insert many elements to test pruning
	for i := 0; i < 100; i++ {
		tree.Insert(uint64(i), i)
	}

	// Should find only nearby elements
	results := tree.FindWithinDistance(50, 2)

	// Verify all results are actually within distance
	for _, idx := range results {
		dist := HammingDistance(50, uint64(idx))
		if dist > 2 {
			t.Errorf("found index %d with distance %d, expected <= 2", idx, dist)
		}
	}
}

func TestBKTree_LargeThreshold(t *testing.T) {
	tree := NewBKTree(HammingDistance)

	for i := 0; i < 10; i++ {
		tree.Insert(uint64(i), i)
	}

	// Large threshold should return all
	results := tree.FindWithinDistance(0, 64)
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

// Benchmark to verify performance improvement
func BenchmarkBKTree_Insert(b *testing.B) {
	tree := NewBKTree(HammingDistance)
	for i := 0; i < b.N; i++ {
		tree.Insert(uint64(i*12345), i)
	}
}

func BenchmarkBKTree_Find(b *testing.B) {
	tree := NewBKTree(HammingDistance)
	for i := 0; i < 10000; i++ {
		tree.Insert(uint64(i*12345), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.FindWithinDistance(uint64(i*67890), 10)
	}
}
