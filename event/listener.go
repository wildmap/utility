package event

import (
	"runtime/debug"
	"slices"
	"sort"

	"github.com/wildmap/utility/xlog"
)

// Listener 事件监听器，封装事件处理逻辑和执行优先级。
//
// 优先级值越小的监听器越先执行，适合实现拦截器链等有序处理场景。
type Listener struct {
	key      string                 // 监听的事件标识符，与 Facade.Fire 的 key 对应
	priority int                    // 执行优先级，值越小越先执行
	consume  func(i map[string]any) // 事件处理回调函数
}

// NewListener 创建事件监听器实例。
func NewListener(key string, priority int, consume func(i map[string]any)) *Listener {
	return &Listener{
		key:      key,
		priority: priority,
		consume:  consume,
	}
}

// onEvent 执行事件处理回调，通过 recover 捕获回调中的 panic，
// 防止单个监听器的异常阻断后续监听器的执行。
func (l *Listener) onEvent(i map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("%s key %d priority listener panic %v\n%s", l.key, l.priority, r, string(debug.Stack()))
		}
	}()
	l.consume(i)
}

// listenerSet 管理同一事件 key 下的所有监听器，支持按优先级有序执行。
//
// 采用懒排序策略：新注册监听器时只标记为未排序（sorted=false），
// 真正触发事件时才排序（consume 方法），避免频繁注册场景下的无效排序开销。
type listenerSet struct {
	listeners []*Listener // 监听器列表
	sorted    bool        // 当前列表是否已按优先级排序
}

// newListenerSet 创建空的监听器集合，初始状态已排序（空集合无需排序）。
func newListenerSet() *listenerSet {
	return &listenerSet{
		listeners: []*Listener{},
		sorted:    true,
	}
}

// register 注册监听器，通过 slices.Contains 防止同一实例重复注册。
//
// 幂等保证：相同监听器实例（指针相等）只会被注册一次，
// 防止事件被多次处理导致业务逻辑错误。
func (set *listenerSet) register(l *Listener) {
	if slices.Contains(set.listeners, l) {
		return
	}

	set.listeners = append(set.listeners, l)
	set.sorted = false // 新增监听器后标记为未排序，触发下次 consume 时重新排序
}

// unregister 注销指定监听器，使用"swap with last + shrink"算法避免内存泄漏。
//
// 将被删除元素的位置用后续元素覆盖，并将末尾元素置 nil，
// 防止 slice 底层数组持有已删除对象的引用导致无法被 GC 回收。
func (set *listenerSet) unregister(l *Listener) {
	for i := 0; i < len(set.listeners); i++ {
		if set.listeners[i] == l {
			// 前移后续元素（保持顺序，而非 swap with last）
			copy(set.listeners[i:], set.listeners[i+1:])
			set.listeners[len(set.listeners)-1] = nil // 显式置 nil，允许 GC 回收
			set.listeners = set.listeners[:len(set.listeners)-1]
			return
		}
	}
}

// consume 触发所有监听器，执行前确保已按优先级排序。
//
// 事件触发前先拷贝监听器快照，防止回调中的注册/注销操作影响当前迭代。
// 这是一种防御性编程实践，以轻微的内存开销换取迭代安全性。
func (set *listenerSet) consume(i map[string]any) {
	if !set.sorted {
		sort.Sort(set)
		set.sorted = true
	}
	// 拷贝快照：防止回调中调用 Register/Unregister 修改原切片导致迭代混乱
	listeners := make([]*Listener, len(set.listeners))
	copy(listeners, set.listeners)

	for _, l := range listeners {
		l.onEvent(i)
	}
}

// Len 实现 sort.Interface，返回监听器数量。
func (set *listenerSet) Len() int {
	return len(set.listeners)
}

// Swap 实现 sort.Interface，交换两个监听器的位置。
func (set *listenerSet) Swap(i, j int) {
	set.listeners[i], set.listeners[j] = set.listeners[j], set.listeners[i]
}

// Less 实现 sort.Interface，按 priority 升序排列（值小的优先执行）。
func (set *listenerSet) Less(i, j int) bool {
	return set.listeners[i].priority < set.listeners[j].priority
}
