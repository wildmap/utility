package event

import (
	"sync"
)

// Facade 事件
type Facade struct {
	mu           sync.RWMutex
	listenerSets map[int]*listenerSet
}

func NewFacade() *Facade {
	return &Facade{
		listenerSets: make(map[int]*listenerSet),
	}
}

// QuickRegister 快速注册
func (e *Facade) QuickRegister(key int, priority int, consume func(i any)) *Listener {
	if consume == nil {
		return nil
	}

	l := NewListener(key, priority, consume)
	e.Register(l)
	return l
}

// Register 注册监听器
func (e *Facade) Register(l *Listener) {
	if l == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	elem, ok := e.listenerSets[l.key]
	if !ok {
		elem = newListenerSet()
		e.listenerSets[l.key] = elem
	}
	elem.register(l)
}

// Unregister 反注册监听器
func (e *Facade) Unregister(l *Listener) {
	if l == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	elem, ok := e.listenerSets[l.key]
	if ok {
		elem.unregister(l)
		// Clean up empty sets to prevent memory leaks
		if elem.Len() == 0 {
			delete(e.listenerSets, l.key)
		}
	}
}

// Fire 抛出事件
func (e *Facade) Fire(key int, input any) {
	e.mu.RLock()
	elem, ok := e.listenerSets[key]
	e.mu.RUnlock()

	if ok {
		elem.consume(input)
	}
}
