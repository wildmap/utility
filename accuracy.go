package utility

import "math"

// 浮点数精度比较工具包
//
// 由于浮点数的二进制表示特性，直接使用 == 或 != 比较浮点数会产生精度误差。
// 本包提供了基于误差阈值的浮点数比较方法，避免精度问题导致的逻辑错误。
//
// 使用场景：
//   - 游戏中的坐标、距离计算
//   - 金融计算中的金额比较
//   - 物理模拟中的数值比较
//
// 注意事项：
//   - 所有比较函数都使用 LowEpsilon 作为默认阈值
//   - 如需更高精度，请使用 Epsilon 常量自行实现
//   - 对于货币等场景，建议使用整数（以分为单位）避免浮点数

const (
	// Epsilon 高精度浮点数比较阈值
	// 值：0.00000001 (10^-8)
	// 适用场景：科学计算、高精度数值模拟
	Epsilon = 0.00000001

	// LowEpsilon 低精度浮点数比较阈值
	// 值：0.01 (10^-2)
	// 适用场景：一般业务逻辑、游戏开发、UI布局计算
	// 注意：本包所有比较函数默认使用此阈值
	LowEpsilon = 0.01
)

// Equal 判断两个浮点数是否近似相等
//
// 比较逻辑：|a - b| < LowEpsilon
//
// 参数：
//
//	a - 第一个浮点数
//	b - 第二个浮点数
//
// 返回值：
//
//	true  - 两数之差的绝对值小于 LowEpsilon，视为相等
//	false - 两数差异超过阈值，视为不等
//
// 示例：
//
//	Equal(0.01, 0.02)   // false，差值 0.01 等于阈值
//	Equal(1.001, 1.002) // true，差值 0.001 小于阈值
//	Equal(0.0, 0.009)   // true，差值在误差范围内
func Equal(a, b float64) bool {
	return math.Abs(a-b) < LowEpsilon
}

// Greater 判断 a 是否明显大于 b
//
// 比较逻辑：a > b 且 (a - b) > LowEpsilon
// 注意：a 必须明显大于 b，不包括近似相等的情况
//
// 参数：
//
//	a - 第一个浮点数
//	b - 第二个浮点数
//
// 返回值：
//
//	true  - a 明显大于 b（差值超过 LowEpsilon）
//	false - a 小于等于 b，或者差值在误差范围内
//
// 示例：
//
//	Greater(1.02, 1.0)  // true，差值 0.02 > LowEpsilon
//	Greater(1.005, 1.0) // false，差值小于阈值
func Greater(a, b float64) bool {
	return math.Max(a, b) == a && math.Abs(a-b) > LowEpsilon
}

// Smaller 判断 a 是否明显小于 b
//
// 比较逻辑：a < b 且 (b - a) > LowEpsilon
// 注意：a 必须明显小于 b，不包括近似相等的情况
//
// 参数：
//
//	a - 第一个浮点数
//	b - 第二个浮点数
//
// 返回值：
//
//	true  - a 明显小于 b（差值超过 LowEpsilon）
//	false - a 大于等于 b，或者差值在误差范围内
//
// 示例：
//
//	Smaller(1.0, 1.02)  // true，差值 0.02 > LowEpsilon
//	Smaller(1.0, 1.005) // false，差值小于阈值
func Smaller(a, b float64) bool {
	return math.Max(a, b) == b && math.Abs(a-b) > LowEpsilon
}

// GreaterOrEqual 判断 a 是否大于等于 b
//
// 比较逻辑：a > b 或 |a - b| < LowEpsilon
// 注意：包括明显大于和近似相等两种情况
//
// 参数：
//
//	a - 第一个浮点数
//	b - 第二个浮点数
//
// 返回值：
//
//	true  - a 大于 b 或两者近似相等
//	false - a 明显小于 b
//
// 示例：
//
//	GreaterOrEqual(1.02, 1.0)  // true，大于
//	GreaterOrEqual(1.005, 1.0) // true，近似相等
//	GreaterOrEqual(0.98, 1.0)  // false，明显小于
func GreaterOrEqual(a, b float64) bool {
	return math.Max(a, b) == a || math.Abs(a-b) < LowEpsilon
}

// SmallerOrEqual 判断 a 是否小于等于 b
//
// 比较逻辑：a < b 或 |a - b| < LowEpsilon
// 注意：包括明显小于和近似相等两种情况
//
// 参数：
//
//	a - 第一个浮点数
//	b - 第二个浮点数
//
// 返回值：
//
//	true  - a 小于 b 或两者近似相等
//	false - a 明显大于 b
//
// 示例：
//
//	SmallerOrEqual(1.0, 1.02)  // true，小于
//	SmallerOrEqual(1.0, 1.005) // true，近似相等
//	SmallerOrEqual(1.02, 1.0)  // false，明显大于
func SmallerOrEqual(a, b float64) bool {
	return math.Max(a, b) == b || math.Abs(a-b) < LowEpsilon
}
