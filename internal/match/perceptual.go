package match

import (
	"imagedupfinder/internal/hash"
	"imagedupfinder/internal/models"
)

// PerceptualMatcher finds groups of similar images using perceptual hashing
type PerceptualMatcher struct {
	threshold int
}

// NewPerceptualMatcher creates a new PerceptualMatcher
func NewPerceptualMatcher(threshold int) *PerceptualMatcher {
	if threshold < 0 {
		threshold = 10 // Default threshold
	}
	return &PerceptualMatcher{threshold: threshold}
}

// FindGroups finds groups of similar images based on Hamming distance.
// Uses BK-Tree for O(n log n) average-case performance instead of O(n²).
func (m *PerceptualMatcher) FindGroups(images []*models.ImageInfo) []*models.DuplicateGroup {
	n := len(images)
	if n < 2 {
		return nil
	}

	// Use Union-Find to group similar images
	uf := newUnionFind(n)

	// Use BK-Tree for efficient similarity search
	tree := newBKTree(hash.HammingDistance)

	for i, img := range images {
		// Find all existing images within threshold distance
		neighbors := tree.findWithinDistance(img.Hash, m.threshold)
		for _, j := range neighbors {
			uf.union(i, j)
		}
		// Add current image to tree
		tree.insert(img.Hash, i)
	}

	// Collect groups
	groupMap := make(map[int][]*models.ImageInfo)
	for i, img := range images {
		root := uf.find(i)
		groupMap[root] = append(groupMap[root], img)
	}

	return buildGroups(groupMap)
}

// GetThreshold returns the current threshold
func (m *PerceptualMatcher) GetThreshold() int {
	return m.threshold
}

// Union-Find data structure for efficient grouping
type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(n int) *unionFind {
	parent := make([]int, n)
	rank := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	return &unionFind{parent: parent, rank: rank}
}

func (uf *unionFind) find(x int) int {
	if uf.parent[x] != x {
		uf.parent[x] = uf.find(uf.parent[x]) // Path compression
	}
	return uf.parent[x]
}

func (uf *unionFind) union(x, y int) {
	px, py := uf.find(x), uf.find(y)
	if px == py {
		return
	}
	// Union by rank
	if uf.rank[px] < uf.rank[py] {
		px, py = py, px
	}
	uf.parent[py] = px
	if uf.rank[px] == uf.rank[py] {
		uf.rank[px]++
	}
}

// bkTree is a BK-tree for efficient similarity search using metric distances.
// It supports O(log n) average-case lookup for finding all elements within
// a given distance threshold.
type bkTree struct {
	root     *bkNode
	distance func(a, b uint64) int
}

type bkNode struct {
	hash     uint64
	index    int
	children map[int]*bkNode // distance -> child node
}

// newBKTree creates a new BK-tree with the given distance function.
func newBKTree(distanceFn func(a, b uint64) int) *bkTree {
	return &bkTree{
		distance: distanceFn,
	}
}

// insert adds a new hash with its associated index to the tree.
func (t *bkTree) insert(hash uint64, index int) {
	node := &bkNode{
		hash:     hash,
		index:    index,
		children: make(map[int]*bkNode),
	}

	if t.root == nil {
		t.root = node
		return
	}

	current := t.root
	for {
		dist := t.distance(hash, current.hash)
		if child, exists := current.children[dist]; exists {
			current = child
		} else {
			current.children[dist] = node
			return
		}
	}
}

// findWithinDistance returns all indices of elements within the given
// distance threshold from the query hash.
func (t *bkTree) findWithinDistance(hash uint64, threshold int) []int {
	if t.root == nil {
		return nil
	}

	var results []int
	t.searchNode(t.root, hash, threshold, &results)
	return results
}

func (t *bkTree) searchNode(node *bkNode, hash uint64, threshold int, results *[]int) {
	dist := t.distance(hash, node.hash)

	if dist <= threshold {
		*results = append(*results, node.index)
	}

	// Triangle inequality: only need to check children with distance
	// in range [dist - threshold, dist + threshold]
	minDist := dist - threshold
	if minDist < 0 {
		minDist = 0
	}
	maxDist := dist + threshold

	for childDist, child := range node.children {
		if childDist >= minDist && childDist <= maxDist {
			t.searchNode(child, hash, threshold, results)
		}
	}
}

// size returns the number of elements in the tree.
func (t *bkTree) size() int {
	if t.root == nil {
		return 0
	}
	return t.countNodes(t.root)
}

func (t *bkTree) countNodes(node *bkNode) int {
	count := 1
	for _, child := range node.children {
		count += t.countNodes(child)
	}
	return count
}
