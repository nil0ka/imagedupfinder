package internal

// BKTree is a BK-tree for efficient similarity search using metric distances.
// It supports O(log n) average-case lookup for finding all elements within
// a given distance threshold.
type BKTree struct {
	root     *bkNode
	distance func(a, b uint64) int
}

type bkNode struct {
	hash     uint64
	index    int
	children map[int]*bkNode // distance -> child node
}

// NewBKTree creates a new BK-tree with the given distance function.
func NewBKTree(distanceFn func(a, b uint64) int) *BKTree {
	return &BKTree{
		distance: distanceFn,
	}
}

// Insert adds a new hash with its associated index to the tree.
func (t *BKTree) Insert(hash uint64, index int) {
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

// FindWithinDistance returns all indices of elements within the given
// distance threshold from the query hash.
func (t *BKTree) FindWithinDistance(hash uint64, threshold int) []int {
	if t.root == nil {
		return nil
	}

	var results []int
	t.searchNode(t.root, hash, threshold, &results)
	return results
}

func (t *BKTree) searchNode(node *bkNode, hash uint64, threshold int, results *[]int) {
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

// Size returns the number of elements in the tree.
func (t *BKTree) Size() int {
	if t.root == nil {
		return 0
	}
	return t.countNodes(t.root)
}

func (t *BKTree) countNodes(node *bkNode) int {
	count := 1
	for _, child := range node.children {
		count += t.countNodes(child)
	}
	return count
}
