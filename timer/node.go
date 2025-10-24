package timer

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/wildmap/utility/list"
)

const (
	// haveStop is a flag indicating the node has been stopped
	haveStop = uint32(1)
)

// Time represents a timer list head with version control and thread-safety.
// Currently implemented using sync.Mutex for functionality.
// TODO: Optimize using CAS (Compare-And-Swap) operations in the future.
//
// Version format (64-bit):
// |---16bit---|---16bit---|------32bit-----|
// |---level---|---index---|-------seq------|
//
// level: indicates which tier of the timewheel this list belongs to
//   - 1 for near list (t1)
//   - 2-5 for T2-T5 lists
//
// index: the slot index within the tier
// seq:   sequence number that increments on each modification
type Time struct {
	timeNode
	sync.Mutex

	// version uses atomic operations to track list modifications
	// Upper 32 bits: (level << 16 | index)
	// Lower 32 bits: sequence number
	version atomic.Uint64
}

// newTimeHead creates a new timer list head with the given level and index.
// The version is initialized with level, index, and sequence number 0.
func newTimeHead(level uint64, index uint64) *Time {
	head := &Time{}
	head.version.Store(genVersion(level, index, 0))
	head.Init()
	return head
}

// genVersion generates a 64-bit version number from level, index, and sequence.
// It ensures each field doesn't overflow its allocated bit range:
// - level: 16 bits (masked with 0xFFFF)
// - index: 16 bits (masked with 0xFFFF)
// - seq:   32 bits (masked with 0xFFFFFFFF)
func genVersion(level uint64, index uint64, seq uint64) uint64 {
	// Ensure each field doesn't overflow its bit allocation
	level = level & 0xFFFF
	index = index & 0xFFFF
	seq = seq & 0xFFFFFFFF
	return level<<48 | index<<32 | seq
}

// lockPushBack adds a node to the tail of the list with proper locking.
// It performs several checks:
// 1. Validates timer and node are not nil
// 2. Checks if node is already stopped
// 3. Updates the node's list pointer and version
// 4. Increments the sequence number in the version
//
// Parameters:
//
//	node: the timer node to add
//	level: the timewheel tier level
//	index: the slot index within the tier
func (t *Time) lockPushBack(node *timeNode, level uint64, index uint64) error {
	if t == nil {
		return ErrNilTimer
	}
	if node == nil {
		return ErrNilNode
	}

	t.Lock()
	defer t.Unlock()

	// Check if node is already stopped
	if node.stop.Load() == haveStop {
		return ErrNodeStopped
	}

	// Add node to the tail of the list
	t.AddTail(&node.Head)
	// Update node's list pointer atomically
	atomic.StorePointer(&node.list, unsafe.Pointer(t))

	// Increment sequence number in version
	currentVersion := t.version.Load()
	seq := (currentVersion & 0xFFFFFFFF) + 1
	newVersion := genVersion(level, index, seq)
	t.version.Store(newVersion)

	// Update node's version to match the list version
	node.version.Store(newVersion)

	return nil
}

// timeNode represents a single timer node in the timewheel.
// It tracks expiration time, callback function, and its position in the list.
type timeNode struct {
	expire     uint64         // Absolute expiration time in ticks
	userExpire time.Duration  // User-specified duration (for periodic timers)
	callback   func()         // Function to call when timer expires
	stop       atomic.Uint32  // Stop flag (0=running, haveStop=stopped)
	list       unsafe.Pointer // Pointer to the Time (list head) this node belongs to
	version    atomic.Uint64  // Version number matching the list's version
	isSchedule bool           // True if this is a periodic timer
	root       *timeWheel     // Reference to the parent timewheel
	list.Head                 // Embedded doubly-linked list node
}

// Stop attempts to stop the timer and remove it from the list.
// A timeNode has 4 possible states:
// 1. In the initial list (safe to stop with lock)
// 2. Moved to tmp list during cascade (race condition possible)
// 3.1. Moved to new list after cascade (safe to stop with lock)
// 3.2. Being executed directly (race condition possible)
//
// States 1 and 3.1 are protected by locks and safe.
// States 2 and 3.2 have potential data races.
//
// This implementation uses a version-based algorithm:
// - If node.version == list.version: node hasn't been moved, safe to delete
// - If node.version != list.version: node has been moved, use lazy deletion
//
// Returns:
//
//	true if the node was successfully stopped and removed
//	false if already stopped or if lazy deletion is used
func (t *timeNode) Stop() bool {
	if t == nil {
		return false
	}

	// Set stop flag atomically - only succeeds once
	if !t.stop.CompareAndSwap(0, haveStop) {
		// Already stopped
		return false
	}

	// Get the list this node belongs to
	cpyList := (*Time)(atomic.LoadPointer(&t.list))
	if cpyList == nil {
		return false
	}

	cpyList.Lock()
	defer cpyList.Unlock()

	// Version check: if versions don't match, node has been moved
	if t.version.Load() != cpyList.version.Load() {
		// Node has been moved (state 2 or 3.2)
		// Use lazy deletion - just return, node will be skipped during execution
		return false
	}

	// Node is still in this list (state 1 or 3.1) - safe to delete
	cpyList.Del(&t.Head)
	return true
}

// Reset resets the timer with a new expiration duration.
// It removes the node from its current list and re-adds it with the new expiration.
//
// The reset process:
// 1. Validates the node and expiration
// 2. Acquires lock on current list
// 3. Checks version to ensure node hasn't been moved
// 4. Removes node from current list
// 5. Resets stop flag
// 6. Recalculates expiration time
// 7. Re-adds node to appropriate timewheel slot
//
// Parameters:
//
//	expire: new duration until timer expiration
//
// Returns:
//
//	(true, nil) if reset was successful
//	(false, error) if reset failed with reason
func (t *timeNode) Reset(expire time.Duration) (bool, error) {
	if t == nil {
		return false, ErrNilNode
	}

	if expire <= 0 {
		return false, newError("Reset", ErrInvalidExpire, "expire must be greater than 0")
	}

	if t.root == nil {
		return false, newError("Reset", ErrNoRoot, "node has no associated timewheel")
	}

	cpyList := (*Time)(atomic.LoadPointer(&t.list))
	if cpyList == nil {
		return false, newError("Reset", ErrNilTimer, "node list is nil")
	}

	cpyList.Lock()
	// Check version to ensure node hasn't been moved
	if t.version.Load() != cpyList.version.Load() {
		cpyList.Unlock()
		return false, ErrNodeMoved
	}

	// Remove from current list
	cpyList.Del(&t.Head)
	cpyList.Unlock()

	// Reset stop flag to allow timer to run again
	t.stop.Store(0)

	// Recalculate absolute expiration time
	jiffies := atomic.LoadUint64(&t.root.jiffies)
	t.expire = uint64(expire/timeUnit) + jiffies

	// Re-add to timewheel at appropriate slot
	if err := t.root.add(t, jiffies); err != nil {
		return false, newError("Reset", err, "failed to add node back")
	}

	return true, nil
}
