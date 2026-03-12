package utility

import (
	"cmp"
	"sort"
)

// Ordered 所有可排序类型的约束别名，包含 cmp.Ordered 约束所覆盖的所有内置有序类型。
//
// 定义为类型别名而非直接使用 cmp.Ordered，便于在本包内统一表达排序语义，
// 同时保持与标准库的互操作性。
type Ordered cmp.Ordered

// Item 表示 map 中的键值对元素，用于排序后的有序遍历。
//
// 使用结构体封装键值对而非使用匿名接口，保留了类型信息，
// 使调用方可以直接访问强类型的 Key 和 Value 字段，无需类型断言。
type Item[K, V any] struct {
	Key   K
	Value V
}

// Less 定义 map 元素的比较函数类型。
//
// 遵循 sort.Interface 的语义：当 x 应排在 y 之前时返回 true。
// 通过函数参数而非硬编码比较逻辑，使调用方可以自由定义升序、降序或自定义排序规则。
type Less[K, V any] func(x, y Item[K, V]) bool

// flatmap 是 map 转换为可排序切片后的中间结构。
//
// Go 的 map 迭代顺序是随机的，无法直接排序，因此需要先展开为切片。
// 通过实现 sort.Interface 接口，可以复用标准库的排序算法（内省排序）。
type flatmap[K comparable, V any] struct {
	items []Item[K, V] // 展开后的键值对切片
	less  Less[K, V]   // 排序用的比较函数
}

// newFlatMap 将 map 展开为可排序的 flatmap 结构。
//
// 预分配 len(m) 容量的切片避免 append 扩容，减少内存分配次数。
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

// Len 返回元素数量，实现 sort.Interface。
func (m *flatmap[K, V]) Len() int {
	return len(m.items)
}

// Less 判断位置 i 的元素是否应排在位置 j 之前，实现 sort.Interface。
func (m *flatmap[K, V]) Less(i, j int) bool {
	return m.less(m.items[i], m.items[j])
}

// Swap 交换位置 i 和 j 的元素，实现 sort.Interface。
func (m *flatmap[K, V]) Swap(i, j int) {
	m.items[i], m.items[j] = m.items[j], m.items[i]
}

// Items 已排序的键值对切片，提供对排序结果的进一步处理能力。
type Items[K, V any] []Item[K, V]

// Top 返回前 n 个元素的切片视图（共享底层数组，不复制）。
//
// 当 n 超过实际元素数量时，自动截断为全部元素，避免越界 panic。
// 适合在大量数据排序后仅关心 Top-N 结果的场景。
func (r Items[K, V]) Top(n int) Items[K, V] {
	if n > len(r) {
		n = len(r)
	}
	return r[:n]
}

// ByFunc 使用自定义比较函数对 map 进行排序，返回有序的键值对切片。
//
// 内部使用标准库 sort.Sort（内省排序算法），时间复杂度 O(n log n)。
// 返回的切片按 less 函数定义的顺序排列，调用方可直接遍历使用。
func ByFunc[K comparable, V any](m map[K]V, c Less[K, V]) Items[K, V] {
	fm := newFlatMap(m, c)
	sort.Sort(fm)
	return fm.items
}

// ByKey 按键的自然升序排序 map（键必须满足 Ordered 约束）。
//
// 利用标准库 cmp.Less 实现与 Go 内置排序一致的比较语义，
// 对字符串键会按字典序升序排列，对数值键会按数值升序排列。
func ByKey[K Ordered, V any](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(x.Key, y.Key)
	})
}

// ByKeyDesc 按键的自然降序排序 map（键必须满足 Ordered 约束）。
//
// 通过交换 cmp.Less 的参数顺序实现降序，无需额外实现。
func ByKeyDesc[K Ordered, V any](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(y.Key, x.Key)
	})
}

// ByValue 按值的自然升序排序 map（值必须满足 Ordered 约束）。
//
// 典型用途：按计数值从小到大排序统计结果。
func ByValue[K comparable, V Ordered](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(x.Value, y.Value)
	})
}

// ByValueDesc 按值的自然降序排序 map（值必须满足 Ordered 约束）。
//
// 典型用途：按得分从高到低排序排行榜数据。
func ByValueDesc[K comparable, V Ordered](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(y.Value, x.Value)
	})
}
