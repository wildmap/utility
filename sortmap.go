package utility

import (
	"cmp"
	"sort"
)

// Ordered 定义所有可排序类型的类型约束
// 包含所有实现cmp.Ordered接口的类型
type Ordered cmp.Ordered

// Item 表示map中的键值对元素
// K是键的类型, V是值的类型
type Item[K, V any] struct {
	Key   K // 键
	Value V // 值
}

// Less map元素的比较函数类型
// 如果x应该排在y之前则返回true
type Less[K, V any] func(x, y Item[K, V]) bool

// flatmap 用于排序的扁平化map结构
// 将map转换为可排序的Item切片
type flatmap[K comparable, V any] struct {
	items []Item[K, V] // 键值对切片
	less  Less[K, V]   // 排序用的比较函数
}

// newFlatMap 从map和比较函数创建新的flatmap实例
// 将所有map条目转换为Item切片以便排序
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

// Len 返回flatmap中的元素数量
// sort.Interface要求的方法
func (m *flatmap[K, V]) Len() int {
	return len(m.items)
}

// Less 报告索引i的元素是否应该排在索引j之前
// sort.Interface要求的方法
func (m *flatmap[K, V]) Less(i, j int) bool {
	return m.less(m.items[i], m.items[j])
}

// Swap 交换索引i和j处的元素
// sort.Interface要求的方法
func (m *flatmap[K, V]) Swap(i, j int) {
	m.items[i], m.items[j] = m.items[j], m.items[i]
}

// Items map元素(键值对)的切片
// 提供用于处理已排序map结果的实用方法
type Items[K, V any] []Item[K, V]

// Top 返回最多包含前n个元素的切片
// 如果n大于Items长度,返回所有元素
func (r Items[K, V]) Top(n int) Items[K, V] {
	if n > len(r) {
		n = len(r)
	}
	return r[:n]
}

// ByFunc 使用提供的比较函数对map进行排序
// 参数: m - 要排序的map, c - 比较函数
// 返回: 已排序的Items切片
func ByFunc[K comparable, V any](m map[K]V, c Less[K, V]) Items[K, V] {
	fm := newFlatMap(m, c)
	sort.Sort(fm)
	return fm.items
}

// ByKey 按键升序对map排序
// K必须是Ordered类型(支持比较)
func ByKey[K Ordered, V any](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(x.Key, y.Key)
	})
}

// ByKeyDesc 按键降序对map排序
// K必须是Ordered类型(支持比较)
func ByKeyDesc[K Ordered, V any](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(y.Key, x.Key)
	})
}

// ByValue 按值升序对map排序
// V必须是Ordered类型(支持比较)
func ByValue[K comparable, V Ordered](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(x.Value, y.Value)
	})
}

// ByValueDesc 按值降序对map排序
// V必须是Ordered类型(支持比较)
func ByValueDesc[K comparable, V Ordered](m map[K]V) Items[K, V] {
	return ByFunc(m, func(x, y Item[K, V]) bool {
		return cmp.Less(y.Value, x.Value)
	})
}
