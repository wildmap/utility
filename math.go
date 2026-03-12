package utility

import "cmp"

// Integer 泛型数值类型约束，涵盖所有有符号整数和浮点数类型。
//
// 使用 ~ 前缀表示底层类型约束，允许基于这些类型定义的自定义类型也满足此约束，
// 增强了泛型函数对用户自定义数值类型的兼容性。
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float32 | ~float64
}

// Abs 返回任意数值类型的绝对值。
//
// 通过泛型实现，避免为每种数值类型单独编写重载函数。
// 注意：对 math.MinInt 取绝对值会发生溢出，调用方应自行处理边界情况。
func Abs[T Integer](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

// FindIdx 在切片中线性搜索目标值，返回首次出现的索引。
//
// 适用于无序切片的小规模查找（O(n)）。
// 对于有序切片，建议使用 sort.SearchInts 等二分查找函数以获得 O(log n) 性能。
// 未找到目标值时返回 -1，与 strings.Index 等标准库保持一致的惯例。
func FindIdx[T cmp.Ordered](aim []T, value T) int {
	for i, v := range aim {
		if v == value {
			return i
		}
	}
	return -1
}

// Filter 基于谓词函数过滤切片，返回所有满足条件的元素组成的新切片。
//
// 惰性分配：结果切片通过 append 动态扩容，避免预分配过多内存。
// 原切片保持不变，适合函数式风格的数据处理管道。
func Filter[T any](slice []T, predicate func(T) bool) []T {
	var filtered []T
	for _, item := range slice {
		if predicate(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
