package utility

// NewTopN creates a new TopN instance with specified capacity and comparator
// n: maximum number of elements to maintain
// comparer: comparison function that returns negative if a < b, zero if a == b, positive if a > b
func NewTopN[T any](n int, comparer func(a, b T) int) *TopN[T] {
	return &TopN[T]{
		n:        n,
		comparer: comparer,
		slice:    make([]T, 0, n),
	}
}

// TopN maintains the top N largest (or smallest) elements based on the comparator
// It keeps elements sorted and automatically discards elements that don't qualify
type TopN[T any] struct {
	n        int              // Maximum number of elements to maintain
	slice    []T              // Sorted slice of elements
	comparer func(a, b T) int // Comparison function for ordering elements
}

// Size returns the current number of elements stored in TopN
// The size will be at most n, but may be less if fewer items have been added
func (t *TopN[T]) Size() int {
	return len(t.slice)
}

// Get retrieves the element at the specified index
func (t *TopN[T]) Get(i int) (T, bool) {
	var zero T
	if i < 0 || i >= t.Size() {
		return zero, false
	}
	return t.slice[i], true
}

// Put inserts a new element into the TopN collection
// The element will be inserted in the correct sorted position
// If the collection is full and the new element doesn't qualify, it will be discarded
func (t *TopN[T]) Put(item T) {
	t.slice = t.put(t.slice, item, t.n, t.comparer)
}

// Range iterates through all elements in order
// The function f is called for each element with its index and value
// If f returns true, the iteration stops immediately
func (t *TopN[T]) Range(f func(i int, item T) (abort bool)) {
	for i, item := range t.slice {
		if f(i, item) {
			break
		}
	}
}

// GetAll returns a copy of all elements currently stored
// The returned slice is a new copy, so modifications won't affect the TopN collection
func (t *TopN[T]) GetAll() []T {
	result := make([]T, len(t.slice))
	copy(result, t.slice)
	return result
}

// put inserts an element into the TopN slice maintaining sorted order
// It performs binary search from the end to find the correct insertion position
// Only keeps the top n elements based on the comparator
func (t *TopN[T]) put(slice []T, item T, n int, comparer func(a, b T) int) []T {
	var i int
	var length = len(slice)

	// Find insertion position by searching from back to front
	// This assumes the slice is already sorted in descending order (largest first)
	for i = length - 1; i >= 0; i-- {
		if comparer(item, slice[i]) > 0 {
			break
		}
	}
	i++

	// Only insert if the element qualifies to be in the top N
	if i < n {
		var end = n - 1
		if end > length {
			end = length
		}
		slice = append(slice[:i], append([]T{item}, slice[i:end]...)...)
	}
	return slice
}
