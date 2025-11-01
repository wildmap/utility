package event

import (
	"sync"
)

// Facade is a thread-safe event dispatcher that manages event listeners
// and provides mechanisms to register, unregister, and trigger events.
// It uses a map to organize listeners by event keys, with each key
// mapping to a set of listeners that respond to that specific event.
type Facade struct {
	mu           sync.RWMutex            // Read-write mutex for thread-safe access to listener sets
	listenerSets map[int]*listenerSet    // Map of event keys to their corresponding listener sets
}

// NewFacade creates and initializes a new event manager instance.
// Returns a pointer to a Facade with an empty listener set map ready for use.
func NewFacade() *Facade {
	return &Facade{
		listenerSets: make(map[int]*listenerSet),
	}
}

// QuickRegister provides a convenient way to register an event listener
// by creating a Listener instance from the provided parameters and registering it.
//
// Parameters:
//   - key: The event identifier that this listener will respond to
//   - priority: Execution priority (lower values execute first)
//   - consume: The callback function to execute when the event fires
//
// Returns:
//   - A pointer to the newly created and registered Listener, or nil if consume is nil
func (e *Facade) QuickRegister(key int, priority int, consume func(i any)) *Listener {
	if consume == nil {
		return nil
	}

	l := NewListener(key, priority, consume)
	e.Register(l)
	return l
}

// Register adds a listener to the event manager in a thread-safe manner.
// If the listener's event key doesn't exist in the map, a new listener set
// is created. The method is safe for concurrent use by multiple goroutines.
//
// Parameters:
//   - l: The listener to register (no action taken if nil)
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

// Unregister removes a listener from the event manager in a thread-safe manner.
// If the removal results in an empty listener set, the set is deleted from the map
// to prevent memory leaks. Safe for concurrent use by multiple goroutines.
//
// Parameters:
//   - l: The listener to unregister (no action taken if nil)
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

// Fire synchronously triggers all listeners registered for the specified event key.
// Listeners are executed in priority order (lower priority values execute first).
// This method is thread-safe and blocks until all listeners have completed execution.
//
// Parameters:
//   - key: The event identifier to trigger
//   - input: The data payload to pass to all registered listeners
func (e *Facade) Fire(key int, input any) {
	e.mu.RLock()
	elem, ok := e.listenerSets[key]
	e.mu.RUnlock()

	if ok {
		elem.consume(input)
	}
}

// FireAsync asynchronously triggers all listeners for the specified event key.
// This method spawns a new goroutine to execute the listeners, allowing the
// caller to continue without blocking. Useful for fire-and-forget event patterns.
//
// Parameters:
//   - key: The event identifier to trigger
//   - input: The data payload to pass to all registered listeners
func (e *Facade) FireAsync(key int, input any) {
	go e.Fire(key, input)
}

// HasListeners checks whether any listeners are registered for a given event key.
// This method is thread-safe and can be used to optimize event firing logic
// by avoiding unnecessary processing when no listeners exist.
//
// Parameters:
//   - key: The event identifier to check
//
// Returns:
//   - true if at least one listener is registered for the key, false otherwise
func (e *Facade) HasListeners(key int) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	elem, ok := e.listenerSets[key]
	return ok && elem.Len() > 0
}

// Clear removes all registered listeners from the event manager.
// This is useful for cleanup or reset scenarios. The method is thread-safe.
func (e *Facade) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.listenerSets = make(map[int]*listenerSet)
}
