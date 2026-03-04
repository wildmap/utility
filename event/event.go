package event

// Facade 进程内轻量级事件总线，实现发布/订阅（Pub/Sub）模式。
//
// 使用 map 按事件 key 分组管理监听器集合，支持同一事件注册多个监听器，
// 并通过 listenerSet 内部的优先级排序控制监听器的执行顺序。
// 适合模块内低耦合的事件驱动架构，不跨 goroutine，无并发安全保证。
type Facade struct {
	listenerSets map[string]*listenerSet // 事件 key → 监听器集合的路由表
}

// NewFacade 创建事件总线实例。
func NewFacade() *Facade {
	return &Facade{
		listenerSets: make(map[string]*listenerSet),
	}
}

// QuickRegister 便捷注册接口：创建监听器并立即注册到事件总线。
//
// 相比分别调用 NewListener + Register，减少模板代码，
// 适合简单的内联回调注册场景。consume 为 nil 时跳过注册并返回 nil。
func (e *Facade) QuickRegister(key string, priority int, consume func(i map[string]any)) *Listener {
	if consume == nil {
		return nil
	}

	l := NewListener(key, priority, consume)
	e.Register(l)
	return l
}

// Register 注册监听器到事件总线，同一监听器实例只会注册一次（幂等）。
//
// 若该 key 的监听器集合尚不存在，则自动创建。
// 新注册的监听器会将集合的 sorted 标记置为 false，
// 下次 Fire 时会触发重新排序（延迟排序策略）。
func (e *Facade) Register(l *Listener) {
	if l == nil {
		return
	}

	elem, ok := e.listenerSets[l.key]
	if !ok {
		elem = newListenerSet()
		e.listenerSets[l.key] = elem
	}
	elem.register(l)
}

// Unregister 从事件总线注销监听器。
//
// 注销后自动清理空的监听器集合，防止 key 长期累积导致内存泄漏，
// 这对于动态注册/注销场景（如对象生命周期与监听器绑定）尤为重要。
func (e *Facade) Unregister(l *Listener) {
	if l == nil {
		return
	}

	elem, ok := e.listenerSets[l.key]
	if ok {
		elem.unregister(l)
		// 集合为空时从 map 中删除，避免空集合长期占用内存
		if elem.Len() == 0 {
			delete(e.listenerSets, l.key)
		}
	}
}

// Fire 触发指定 key 的事件，按优先级顺序调用所有已注册的监听器。
//
// 若该 key 没有任何监听器，则静默忽略，不会产生错误。
// input 数据以 map 形式传递，保持灵活性，避免为每种事件定义独立结构体。
func (e *Facade) Fire(key string, input map[string]any) {
	elem, ok := e.listenerSets[key]
	if ok {
		elem.consume(input)
	}
}
