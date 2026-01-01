package internal

import (
	"sort"
)

// Grouper finds groups of similar images
type Grouper struct {
	threshold int // Hamming distance threshold
}

// NewGrouper creates a new Grouper
func NewGrouper(threshold int) *Grouper {
	if threshold < 0 {
		threshold = 10 // Default threshold
	}
	return &Grouper{threshold: threshold}
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

// FindGroups finds groups of similar images based on Hamming distance
func (g *Grouper) FindGroups(images []*ImageInfo) []*DuplicateGroup {
	n := len(images)
	if n < 2 {
		return nil
	}

	// Use Union-Find to group similar images
	uf := newUnionFind(n)

	// Compare all pairs (O(n²) but necessary for correctness)
	// For very large sets, consider using LSH or VP-Tree
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			dist := HammingDistance(images[i].Hash, images[j].Hash)
			if dist <= g.threshold {
				uf.union(i, j)
			}
		}
	}

	// Collect groups
	groupMap := make(map[int][]*ImageInfo)
	for i, img := range images {
		root := uf.find(i)
		groupMap[root] = append(groupMap[root], img)
	}

	// Build result (only groups with 2+ images)
	var groups []*DuplicateGroup
	groupID := 1
	for _, imgs := range groupMap {
		if len(imgs) < 2 {
			continue
		}

		group := &DuplicateGroup{
			ID:     groupID,
			Images: imgs,
		}

		// Determine which image to keep (highest score)
		g.selectKeepAndRemove(group)
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
func (g *Grouper) selectKeepAndRemove(group *DuplicateGroup) {
	if len(group.Images) == 0 {
		return
	}

	// Sort images by score (descending), then by file size (descending),
	// then by mod time (descending), then by path (ascending)
	sorted := make([]*ImageInfo, len(group.Images))
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

// GetThreshold returns the current threshold
func (g *Grouper) GetThreshold() int {
	return g.threshold
}
