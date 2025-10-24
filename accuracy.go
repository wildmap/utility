package utility

import "math"

const (
	// Epsilon defines the normal precision threshold for floating point comparisons
	// This is used for high-precision floating point operations
	Epsilon = 0.00000001

	// LowEpsilon defines the low precision threshold for floating point comparisons
	// This is used for general floating point operations where slight variations are acceptable
	LowEpsilon = 0.01
)

// Equal checks whether floating point value a is equal to b
// Returns true if the absolute difference between a and b is less than LowEpsilon
func Equal(a, b float64) bool {
	return math.Abs(a-b) < LowEpsilon
}

// Greater checks whether floating point value a is greater than b
// Returns true only if a is the maximum value and their difference exceeds LowEpsilon
func Greater(a, b float64) bool {
	return math.Max(a, b) == a && math.Abs(a-b) > LowEpsilon
}

// Smaller checks whether floating point value a is smaller than b
// Returns true only if b is the maximum value and their difference exceeds LowEpsilon
func Smaller(a, b float64) bool {
	return math.Max(a, b) == b && math.Abs(a-b) > LowEpsilon
}

// GreaterOrEqual checks whether floating point value a is greater than or equal to b
// Returns true if a is the maximum value or their difference is within LowEpsilon
func GreaterOrEqual(a, b float64) bool {
	return math.Max(a, b) == a || math.Abs(a-b) < LowEpsilon
}

// SmallerOrEqual checks whether floating point value a is smaller than or equal to b
// Returns true if b is the maximum value or their difference is within LowEpsilon
func SmallerOrEqual(a, b float64) bool {
	return math.Max(a, b) == b || math.Abs(a-b) < LowEpsilon
}
