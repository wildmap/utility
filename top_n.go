package utility

// TopN 维护 Top-N 最大（或最小）元素的有序容器。
//
// 通过自定义比较器定义"最大"的语义，自动维护容量为 n 的有序切片。
// 每次 Put 操作均保持切片有序，超出容量时自动丢弃末位元素。
//
// 时间复杂度：Put 操作为 O(n)（线性插入），Size/Get/Range 为 O(1)。
// 空间复杂度：O(n)，仅保留 n 个元素。
//
// 典型用途：实时维护游戏排行榜 Top-N、统计最热门 N 个词等场景。
type TopN[T any] struct {
	n        int              // 最大保留元素数
	slice    []T              // 有序元素切片，按比较器降序（或自定义序）排列
	comparer func(a, b T) int // 比较函数：a < b 返回负数，a == b 返回 0，a > b 返回正数
}

// NewTopN 创建指定容量的 TopN 容器。
//
// comparer 函数定义了元素的排序语义：
//   - 返回正数表示 a > b（a 优先于 b，排在更前面）
//   - 返回负数表示 a < b（b 优先于 a）
//   - 返回 0 表示 a == b（同等优先级）
//
// 预分配 n 容量的底层切片，避免频繁扩容。
func NewTopN[T any](n int, comparer func(a, b T) int) *TopN[T] {
	return &TopN[T]{
		n:        n,
		comparer: comparer,
		slice:    make([]T, 0, n),
	}
}

// Size 返回当前容器中实际存储的元素数量（≤ n）。
func (t *TopN[T]) Size() int {
	return len(t.slice)
}

// Get 返回指定索引处的元素，索引越界时返回零值和 false。
func (t *TopN[T]) Get(i int) (T, bool) {
	var zero T
	if i < 0 || i >= t.Size() {
		return zero, false
	}
	return t.slice[i], true
}

// Put 将新元素插入到有序切片的正确位置。
//
// 若容器已满且新元素不优于最末尾元素，则直接丢弃；
// 否则在找到正确位置后插入，并截断超出容量的末尾元素。
func (t *TopN[T]) Put(item T) {
	t.slice = t.put(t.slice, item, t.n, t.comparer)
}

// Range 按排名顺序遍历所有元素，当回调函数返回 true 时提前终止遍历。
//
// 提前退出机制（abort=true）允许调用方在找到目标元素后立即停止，
// 避免无谓的全量遍历，对大容量 TopN 有明显性能提升。
func (t *TopN[T]) Range(f func(i int, item T) (abort bool)) {
	for i, item := range t.slice {
		if f(i, item) {
			break
		}
	}
}

// GetAll 返回当前所有元素的深拷贝切片。
//
// 返回独立副本而非切片视图，防止调用方的修改影响内部状态，
// 是防御性编程的体现。
func (t *TopN[T]) GetAll() []T {
	result := make([]T, len(t.slice))
	copy(result, t.slice)
	return result
}

// put 核心插入算法：从末尾向前线性扫描找到插入位置，原地插入并截断。
//
// 算法思路（假设切片按比较器降序排列）：
//  1. 从末尾向前扫描，找到第一个比 item "小"的元素位置 i
//  2. 若 i+1 < n，则在 i+1 处插入 item
//  3. 截断至最多 n 个元素，丢弃末尾不够优秀的元素
//
// 使用 append 实现原地插入，利用已有的底层数组容量避免额外分配。
func (t *TopN[T]) put(slice []T, item T, n int, comparer func(a, b T) int) []T {
	var i int
	var length = len(slice)

	// 从末尾向前找到第一个比 item "小"的位置（item 应插入其后一位）
	for i = length - 1; i >= 0; i-- {
		if comparer(item, slice[i]) > 0 {
			break
		}
	}
	i++ // i 现在是 item 的目标插入位置

	// 仅当插入位置在前 n 名以内时才真正执行插入
	if i < n {
		// end 是有效元素的末尾边界，防止超出当前已有元素范围导致越界
		var end = n - 1
		if end > length {
			end = length
		}
		// 在位置 i 插入 item，同时截断末尾多余的元素
		slice = append(slice[:i], append([]T{item}, slice[i:end]...)...)
	}
	return slice
}
