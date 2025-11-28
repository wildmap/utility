package utility

import "math"

const (
	// Epsilon 浮点正常精度
	Epsilon = 0.00000001

	// LowEpsilon 浮点低精度
	LowEpsilon = 0.01
)

// Equal 相等比较
func Equal(a, b float64) bool {
	return math.Abs(a-b) < LowEpsilon
}

// Greater 大于
func Greater(a, b float64) bool {
	return math.Max(a, b) == a && math.Abs(a-b) > LowEpsilon
}

// Smaller 小于
func Smaller(a, b float64) bool {
	return math.Max(a, b) == b && math.Abs(a-b) > LowEpsilon
}

// GreaterOrEqual 大于等于
func GreaterOrEqual(a, b float64) bool {
	return math.Max(a, b) == a || math.Abs(a-b) < LowEpsilon
}

// SmallerOrEqual 小于等于
func SmallerOrEqual(a, b float64) bool {
	return math.Max(a, b) == b || math.Abs(a-b) < LowEpsilon
}
