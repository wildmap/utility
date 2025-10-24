package event

import (
	"log/slog"
	"sort"
	"sync"
)

// Listener represents an individual event handler that responds to specific events.
// Each listener has a unique combination of event key, priority, and callback function.
type Listener struct {
	key      int           // The event identifier this listener responds to
	priority int           // Execution priority (lower values execute first)
	consume  func(i any)   // The callback function invoked when the event fires
}

// NewListener creates and initializes a new event listener instance.
//
// Parameters:
//   - key: The event identifier this listener will respond to
//   - priority: Execution priority for ordering (lower values execute first)
//   - consume: The callback function to execute when the event is triggered
//
// Returns:
//   - A pointer to the newly created Listener
func NewListener(key int, priority int, consume func(i any)) *Listener {
	return &Listener{
		key:      key,
		priority: priority,
		consume:  consume,
	}
}

// onEvent executes the listener's callback function with panic recovery.
// If the callback panics, the panic is caught and logged, preventing
// it from crashing the entire event processing chain.
//
// Parameters:
//   - i: The event data payload to pass to the callback function
func (l *Listener) onEvent(i any) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("listener panic",
				"key", l.key,
				"priority", l.priority,
				"error", r,
			)
		}
	}()
	l.consume(i)
}

// ------------------------------------------------------------------------------

// listenerSet manages a thread-safe collection of event listeners.
// It maintains listeners in priority order and implements the sort.Interface
// for efficient priority-based sorting. The sorted flag optimizes performance
// by avoiding unnecessary re-sorting when the collection hasn't changed.
type listenerSet struct {
	mu        sync.RWMutex   // Read-write mutex for thread-safe access
	listeners []*Listener    // Slice of registered listeners
	sorted    bool           // Flag indicating whether listeners are currently sorted by priority
}

// newListenerSet creates and initializes a new listener set with
// pre-allocated capacity for performance optimization.
//
// Returns:
//   - A pointer to the newly created listenerSet
func newListenerSet() *listenerSet {
	return &listenerSet{
		listeners: make([]*Listener, 0, 4), // Pre-allocate capacity to reduce allocations
		sorted:    true,
	}
}

// register adds a listener to the set in a thread-safe manner.
// Duplicate registrations of the same listener instance are ignored
// to prevent multiple executions. Adding a listener invalidates the
// sorted flag, triggering a re-sort on the next consume operation.
//
// Parameters:
//   - l: The listener to add to the set
func (set *listenerSet) register(l *Listener) {
	set.mu.Lock()
	defer set.mu.Unlock()

	// Prevent duplicate registration of the same listener instance
	for _, existing := range set.listeners {
		if existing == l {
			return
		}
	}

	set.listeners = append(set.listeners, l)
	set.sorted = false
}

// unregister removes a listener from the set in a thread-safe manner.
// Uses an efficient in-place deletion technique that avoids memory leaks
// by explicitly niling the removed element before reslicing.
//
// Parameters:
//   - l: The listener to remove from the set
func (set *listenerSet) unregister(l *Listener) {
	set.mu.Lock()
	defer set.mu.Unlock()

	for i := 0; i < len(set.listeners); i++ {
		if set.listeners[i] == l {
			// Efficient deletion that prevents memory leaks
			copy(set.listeners[i:], set.listeners[i+1:])
			set.listeners[len(set.listeners)-1] = nil // Prevent memory leak
			set.listeners = set.listeners[:len(set.listeners)-1]
			return
		}
	}
}

// consume triggers all registered listeners with the provided event data.
// Listeners are executed in priority order (lower priority values first).
// The method creates a copy of the listener slice before releasing the lock,
// preventing issues if listeners are added/removed during execution.
//
// Parameters:
//   - i: The event data payload to pass to each listener
func (set *listenerSet) consume(i any) {
	set.mu.Lock()
	if !set.sorted {
		sort.Sort(set)
		set.sorted = true
	}
	// Create a copy to avoid modification issues during execution
	listeners := make([]*Listener, len(set.listeners))
	copy(listeners, set.listeners)
	set.mu.Unlock()

	// Execute listeners in priority order
	for _, l := range listeners {
		l.onEvent(i)
	}
}

// Len returns the number of listeners in the set.
// Implements the sort.Interface.
//
// Returns:
//   - The count of registered listeners
func (set *listenerSet) Len() int {
	return len(set.listeners)
}

// Swap exchanges the positions of two listeners at the given indices.
// Implements the sort.Interface.
//
// Parameters:
//   - i, j: The indices of the listeners to swap
func (set *listenerSet) Swap(i, j int) {
	set.listeners[i], set.listeners[j] = set.listeners[j], set.listeners[i]
}

// Less determines the ordering of listeners based on their priority values.
// Implements the sort.Interface. Listeners are sorted in ascending priority
// order, meaning lower priority values execute first.
//
// Parameters:
//   - i, j: The indices of the listeners to compare
//
// Returns:
//   - true if listener at index i should come before listener at index j
func (set *listenerSet) Less(i, j int) bool {
	return set.listeners[i].priority < set.listeners[j].priority
}
