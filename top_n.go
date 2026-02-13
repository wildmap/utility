package utility

// NewTopN 创建一个新的TopN实例
// 参数:
//
//	n - 要维护的最大元素数量
//	comparer - 比较函数,如果a<b返回负数,a==b返回0,a>b返回正数
//
// 返回: TopN实例指针
func NewTopN[T any](n int, comparer func(a, b T) int) *TopN[T] {
	return &TopN[T]{
		n:        n,
		comparer: comparer,
		slice:    make([]T, 0, n),
	}
}

// TopN 维护前N个最大(或最小)元素
// 基于比较器保持元素有序,自动丢弃不符合条件的元素
// 泛型实现,可用于任何类型
type TopN[T any] struct {
	n        int              // 要维护的最大元素数量
	slice    []T              // 已排序的元素切片
	comparer func(a, b T) int // 元素排序用的比较函数
}

// Size 返回TopN中当前存储的元素数量
// 大小最多为n,但如果添加的元素较少可能会更小
func (t *TopN[T]) Size() int {
	return len(t.slice)
}

// Get 获取指定索引处的元素
// 参数: i - 元素索引(从0开始)
// 返回: 元素值和是否成功的布尔值
func (t *TopN[T]) Get(i int) (T, bool) {
	var zero T
	if i < 0 || i >= t.Size() {
		return zero, false
	}
	return t.slice[i], true
}

// Put 将新元素插入TopN集合
// 元素将插入到正确的有序位置
// 如果集合已满且新元素不符合条件,将被丢弃
func (t *TopN[T]) Put(item T) {
	t.slice = t.put(t.slice, item, t.n, t.comparer)
}

// Range 按顺序遍历所有元素
// 对每个元素调用函数f,传入索引和值
// 如果f返回true,立即停止迭代
func (t *TopN[T]) Range(f func(i int, item T) (abort bool)) {
	for i, item := range t.slice {
		if f(i, item) {
			break
		}
	}
}

// GetAll 返回当前存储的所有元素的副本
// 返回的切片是新副本,修改不会影响TopN集合
func (t *TopN[T]) GetAll() []T {
	result := make([]T, len(t.slice))
	copy(result, t.slice)
	return result
}

// put 将元素插入TopN切片并保持有序
// 从后向前执行二分查找以找到正确的插入位置
// 只保留前n个元素
func (t *TopN[T]) put(slice []T, item T, n int, comparer func(a, b T) int) []T {
	var i int
	var length = len(slice)

	// 从后向前查找插入位置
	// 假设切片已按降序排序(最大值在前)
	for i = length - 1; i >= 0; i-- {
		if comparer(item, slice[i]) > 0 {
			break
		}
	}
	i++

	// 只有当元素符合前N名条件时才插入
	if i < n {
		var end = n - 1
		if end > length {
			end = length
		}
		slice = append(slice[:i], append([]T{item}, slice[i:end]...)...)
	}
	return slice
}
