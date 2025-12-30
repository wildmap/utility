package utility

import "cmp"

// Integer is a generic type constraint that includes all integer and floating point types
// This allows functions to work with any numeric type
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float32 | ~float64
}

// Abs returns the absolute value of the given number
// Generic function that works with any type satisfying the Integer constraint
func Abs[T Integer](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

// FindIdx searches for a value in a slice and returns its index
// Returns the zero-based index if found, or -1 if the value doesn't exist in the slice
// Generic function that works with any ordered type (integers, floats, strings, etc.)
func FindIdx[T cmp.Ordered](aim []T, value T) int {
	for i, v := range aim {
		if v == value {
			return i
		}
	}
	return -1
}

// Filter filters a slice based on a predicate function
func Filter[T any](slice []T, predicate func(T) bool) []T {
	var filtered []T
	for _, item := range slice {
		if predicate(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}