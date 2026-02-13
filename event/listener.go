package event

import (
	"runtime/debug"
	"sort"

	"github.com/wildmap/utility/xlog"
)

// Listener 监听器
type Listener struct {
	key      string                 // The event identifier this listener responds to
	priority int                    // Execution priority (lower values execute first)
	consume  func(i map[string]any) // The callback function invoked when the event fires
}

func NewListener(key string, priority int, consume func(i map[string]any)) *Listener {
	return &Listener{
		key:      key,
		priority: priority,
		consume:  consume,
	}
}

func (l *Listener) onEvent(i map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("%s key %d priority listener panic %v\n%s", l.key, l.priority, r, string(debug.Stack()))
		}
	}()
	l.consume(i)
}

// ------------------------------------------------------------------------------

// 监听器集合
type listenerSet struct {
	listeners []*Listener
	sorted    bool
}

func newListenerSet() *listenerSet {
	return &listenerSet{
		listeners: []*Listener{},
		sorted:    true,
	}
}

func (set *listenerSet) register(l *Listener) {
	// 防止重复注册同一个监听器实例
	for _, existing := range set.listeners {
		if existing == l {
			return
		}
	}

	set.listeners = append(set.listeners, l)
	set.sorted = false
}

func (set *listenerSet) unregister(l *Listener) {
	for i := 0; i < len(set.listeners); i++ {
		if set.listeners[i] == l {
			// 高效删除，防止内存泄漏
			copy(set.listeners[i:], set.listeners[i+1:])
			set.listeners[len(set.listeners)-1] = nil // 防止内存泄漏
			set.listeners = set.listeners[:len(set.listeners)-1]
			return
		}
	}
}

func (set *listenerSet) consume(i map[string]any) {
	if !set.sorted {
		sort.Sort(set)
		set.sorted = true
	}
	// Create a copy to avoid modification issues during execution
	listeners := make([]*Listener, len(set.listeners))
	copy(listeners, set.listeners)

	// 按优先级顺序执行监听器
	for _, l := range listeners {
		l.onEvent(i)
	}
}

// Len implement sort.Interface
func (set *listenerSet) Len() int {
	return len(set.listeners)
}

// Swap implement sort.Interface
func (set *listenerSet) Swap(i, j int) {
	set.listeners[i], set.listeners[j] = set.listeners[j], set.listeners[i]
}

// Less implement sort.Interface
func (set *listenerSet) Less(i, j int) bool {
	return set.listeners[i].priority < set.listeners[j].priority
}
