package utility

import "math"

// 浮点数精度比较工具包
//
// 由于浮点数的 IEEE 754 二进制表示特性，直接使用 == 或 != 运算符比较浮点数
// 会因精度损失导致逻辑错误。本包基于误差阈值（epsilon）实现安全可靠的浮点数比较。
//
// 使用场景：
//   - 游戏中的坐标、距离、碰撞检测计算
//   - 金融计算中的金额对比
//   - 物理模拟中的数值收敛判断
//
// 注意事项：
//   - 所有比较函数均以 LowEpsilon 作为默认误差阈值
//   - 如需更高精度，请使用 Epsilon 常量自行实现比较逻辑
//   - 货币等精确场景建议改用整数（以"分"为单位）彻底规避浮点数问题

const (
	// Epsilon 高精度浮点数误差阈值（10^-8）。
	// 适用于科学计算、高精度数值模拟等对精度要求极高的场景。
	Epsilon = 0.00000001

	// LowEpsilon 低精度浮点数误差阈值（10^-2）。
	// 适用于一般业务逻辑、游戏开发、UI 布局等对精度要求适中的场景。
	// 本包所有比较函数默认使用此阈值。
	LowEpsilon = 0.01
)

// Equal 判断两个浮点数是否在误差范围内近似相等。
//
// 当两数之差的绝对值小于 LowEpsilon 时视为相等，
// 有效规避 IEEE 754 精度损失导致的"0.1 + 0.2 != 0.3"等经典问题。
func Equal(a, b float64) bool {
	return math.Abs(a-b) < LowEpsilon
}

// Greater 判断 a 是否明显大于 b，差值须超过 LowEpsilon 阈值。
//
// 区别于直接使用 > 运算符：当两数差值处于误差范围内时返回 false，
// 防止将近似相等的值误判为大于关系。
func Greater(a, b float64) bool {
	return math.Max(a, b) == a && math.Abs(a-b) > LowEpsilon
}

// Smaller 判断 a 是否明显小于 b，差值须超过 LowEpsilon 阈值。
//
// 区别于直接使用 < 运算符：当两数差值处于误差范围内时返回 false，
// 防止将近似相等的值误判为小于关系。
func Smaller(a, b float64) bool {
	return math.Max(a, b) == b && math.Abs(a-b) > LowEpsilon
}

// GreaterOrEqual 判断 a 是否大于或近似等于 b。
//
// 满足以下任一条件即返回 true：
//   - a 明显大于 b（差值超过 LowEpsilon）
//   - a 与 b 近似相等（差值在 LowEpsilon 内）
func GreaterOrEqual(a, b float64) bool {
	return math.Max(a, b) == a || math.Abs(a-b) < LowEpsilon
}

// SmallerOrEqual 判断 a 是否小于或近似等于 b。
//
// 满足以下任一条件即返回 true：
//   - a 明显小于 b（差值超过 LowEpsilon）
//   - a 与 b 近似相等（差值在 LowEpsilon 内）
func SmallerOrEqual(a, b float64) bool {
	return math.Max(a, b) == b || math.Abs(a-b) < LowEpsilon
}
