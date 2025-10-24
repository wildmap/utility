package utility

import (
	"cmp"
	"sort"
)

// Ordered defines type constraints for all sortable types
// It includes all types that implement cmp.Ordered interface
type Ordered cmp.Ordered

// Item represents a key-value pair element in a map
// K is the type of the key, V is the type of the value
type Item[K, V any] struct {
	Key   K
	Value V
}

// Less is a comparison function for map elements
// It returns true if x should be ordered before y
type Less[K, V any] func(x, y Item[K, V]) bool

// flatmap is a flattened map structure with a comparator for sorting
// It converts a map into a slice of items that can be sorted
type flatmap[K comparable, V any] struct {
	items []Item[K, V] // Slice of key-value pairs
	less  Less[K, V]   // Comparison function for sorting
}

// newFlatMap creates a new flatmap instance from a map and comparison function
// It converts all map entries into a slice of items for sorting
func newFlatMap[K comparable, V any](m map[K]V, less Less[K, V]) *flatmap[K, V] {
	fm := &flatmap[K, V]{
		items: make([]Item[K, V], 0, len(m)),
		less:  less,
	}
	for k, v := range m {
		fm.items = append(fm.items, Item[K, V]{Key: k, Value: v})
	}
	return fm
}

// Len returns the number of items in the flatmap
// This is required by the sort.Interface
func (m *flatmap[K, V]) Len() int {
	return len(m.items)
}

// Less reports whether the item at index i should sort before the item at index j
// This is required by the sort.Interface
func (m *flatmap[K, V]) Less(i, j int) bool {
	return m.less(m.items[i], m.items[j])
}

// Swap swaps the items at indices i and j
// This is required by the sort.Interface
func (m *flatmap[K, V]) Swap(i, j int) {
	m.items[i], m.items[j] = m.items[j], m.items[i]
}

// Items is a slice of map elements (key-value pairs)
// It provides utility methods for working with sorted map results
type Items[K, V any] []Item[K, V]

// Top returns a slice containing at most the first n elements
// If n is greater than the length of Items, all elements are returned
func (r Items[K, V]) Top(n int) Items[K, V] {
	if n > len(r) {
		n = len(r)
	}
	return r[:n]
}

// ByFunc sorts a map using the provided comparison function
// Returns a sorted slice of key-value pairs based on the custom comparator
func ByFunc[K comparable, V any](m map[K]V, c Less[K, V]) Items[K, V] {
	fm := newFlatMap(m, c)
	sort.Sort(fm)
	return fm.items
}

// ByKey sorts a map by key in ascending order
// K must be an Ordered type (supports comparison)
func ByKey[K Ordered, V any](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(x.Key, y.Key)
	})
}

// ByKeyDesc sorts a map by key in descending order
// K must be an Ordered type (supports comparison)
func ByKeyDesc[K Ordered, V any](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(y.Key, x.Key)
	})
}

// ByValue sorts a map by value in ascending order
// V must be an Ordered type (supports comparison)
func ByValue[K comparable, V Ordered](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(x.Value, y.Value)
	})
}

// ByValueDesc sorts a map by value in descending order
// V must be an Ordered type (supports comparison)
func ByValueDesc[K comparable, V Ordered](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(y.Value, x.Value)
	})
}
