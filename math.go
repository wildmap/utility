package utility

import "cmp"

// Integer 泛型类型约束,包含所有整数和浮点数类型
// 允许函数使用任何数值类型
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float32 | ~float64
}

// Abs 返回给定数字的绝对值
// 泛型函数,可用于任何满足Integer约束的类型
// 参数: x - 任意数值类型
// 返回: x的绝对值
func Abs[T Integer](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

// FindIdx 在切片中搜索值并返回其索引
// 参数: aim - 要搜索的切片, value - 要查找的值
// 返回: 如果找到返回从0开始的索引,未找到返回-1
// 泛型函数,适用于任何有序类型(整数、浮点数、字符串等)
func FindIdx[T cmp.Ordered](aim []T, value T) int {
	for i, v := range aim {
		if v == value {
			return i
		}
	}
	return -1
}

// Filter 基于谓词函数过滤切片
// 参数: slice - 源切片, predicate - 过滤条件函数
// 返回: 包含所有满足条件元素的新切片
func Filter[T any](slice []T, predicate func(T) bool) []T {
	var filtered []T
	for _, item := range slice {
		if predicate(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
