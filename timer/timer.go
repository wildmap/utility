package timer

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/wildmap/utility/list"
)

// Timer interface defines the core operations of a hierarchical timewheel timer.
// It provides methods to schedule one-shot and periodic timer callbacks.
type Timer interface {
	// AfterFunc schedules a one-shot callback after the specified duration
	AfterFunc(expire time.Duration, callback func()) (TimeNoder, error)

	// ScheduleFunc schedules a periodic callback that repeats at the specified interval
	ScheduleFunc(expire time.Duration, callback func()) (TimeNoder, error)

	// Stop stops the timer and all pending callbacks
	Stop()
}

// TimeNoder interface represents a timer node with stop and reset capabilities.
type TimeNoder interface {
	// Stop cancels the timer
	Stop() bool

	// Reset reschedules the timer with a new duration
	Reset(expire time.Duration) (bool, error)
}

const (
	// nearShift defines the bit shift for the near list
	// With 8 bits, we get 256 slots in the near list
	nearShift = 8

	// nearSize is the number of slots in the near list (2^8 = 256)
	nearSize = 1 << nearShift

	// levelShift defines the bit shift for each tier list
	// With 6 bits, we get 64 slots per tier
	levelShift = 6

	// levelSize is the number of slots in each tier list (2^6 = 64)
	levelSize = 1 << levelShift

	// nearMask is used to extract the near list index from jiffies
	nearMask = nearSize - 1

	// levelMask is used to extract the tier list index from jiffies
	levelMask = levelSize - 1

	// timeUnit defines the tick duration (3 milliseconds)
	// Each tick of jiffies represents 3ms
	timeUnit = time.Millisecond * 3
)

// NewTimer creates a new hierarchical timewheel timer.
// The timer uses a 5-level hierarchical structure:
// - Level 1 (t1): 256 slots, each representing 3ms
// - Level 2 (t2): 64 slots, each representing 256 * 3ms
// - Level 3 (t3): 64 slots, each representing 256 * 64 * 3ms
// - Level 4 (t4): 64 slots, each representing 256 * 64^2 * 3ms
// - Level 5 (t5): 64 slots, each representing 256 * 64^3 * 3ms
//
// Parameters:
//
//	ctx: context for cancellation and lifecycle management
//
// Returns:
//
//	Timer interface and error (nil on success)
func NewTimer(ctx context.Context) (Timer, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	t := &timeWheel{}
	t.ctx, t.cancel = context.WithCancel(ctx)

	if err := t.init(); err != nil {
		return nil, newError("NewTimer", err, "failed to initialize")
	}
	go t.run()
	return t, nil
}

// timeWheel implements a hierarchical timing wheel algorithm.
// It's similar to the Linux kernel timer implementation.
//
// The timewheel consists of 5 levels:
// - t1 (near): 256 slots for immediate timers (0-768ms)
// - t2-t5: 4 tiers of 64 slots each for longer timers
//
// As time advances, timers cascade from higher tiers to lower tiers,
// and eventually execute when they reach t1.
type timeWheel struct {
	ctx          context.Context     // Context for cancellation
	cancel       context.CancelFunc  // Cancel function
	jiffies      uint64              // Monotonically increasing tick counter
	t1           [nearSize]*Time     // Level 1: 256 slots for near-term timers
	t2Tot5       [4][levelSize]*Time // Levels 2-5: 4 tiers of 64 slots each
	curTimePoint time.Duration       // Current time point in time units
}

// init initializes all slots in the timewheel.
// Each slot is initialized as a circular doubly-linked list head.
// Returns error if any slot fails to initialize.
func (t *timeWheel) init() error {
	if t == nil {
		return ErrNilTimer
	}

	// Initialize near list (t1) - 256 slots
	for i := 0; i < nearSize; i++ {
		t.t1[i] = newTimeHead(1, uint64(i))
		if t.t1[i] == nil {
			return newError("init", ErrNilTimer, "failed to create t1 head")
		}
	}

	// Initialize tier lists (t2-t5) - 4 tiers × 64 slots
	for i := 0; i < 4; i++ {
		for j := 0; j < levelSize; j++ {
			t.t2Tot5[i][j] = newTimeHead(uint64(i+2), uint64(j))
			if t.t2Tot5[i][j] == nil {
				return newError("init", ErrNilTimer, "failed to create t2tot5 head")
			}
		}
	}

	return nil
}

// maxVal returns the maximum number of ticks this timewheel can handle.
// Calculation: 2^(nearShift + 4*levelShift) - 1
// = 2^(8 + 4*6) - 1 = 2^32 - 1
func (t *timeWheel) maxVal() uint64 {
	return (1 << (nearShift + 4*levelShift)) - 1
}

// levelMax returns the maximum tick value for a given tier level.
// index: tier index (0-3 for t2-t5)
// Returns: 2^(nearShift + index*levelShift)
func (t *timeWheel) levelMax(index int) uint64 {
	if index < 0 {
		return 0
	}
	return 1 << (nearShift + index*levelShift)
}

// index calculates the slot index for tier n.
// It extracts the appropriate bits from jiffies based on the tier level.
// n: tier index (0-3 for t2-t5)
// Returns: slot index (0-63) and error
func (t *timeWheel) index(n int) (uint64, error) {
	if n < 0 || n >= 4 {
		return 0, newError("index", ErrIndexOutOfRange, "n must be in range [0, 3]")
	}
	return (t.jiffies >> (nearShift + levelShift*n)) & levelMask, nil
}

// add inserts a timer node into the appropriate slot of the timewheel.
// It determines which tier and slot based on the expiration time.
//
// The algorithm:
// 1. Calculate ticks until expiration (idx = expire - jiffies)
// 2. If idx < 256: add to t1[expire & nearMask]
// 3. Otherwise, find appropriate tier (t2-t5) based on idx
// 4. Add to that tier's slot
//
// Parameters:
//
//	node: the timer node to add
//	jiffies: current tick count
//
// Returns: error if addition fails
func (t *timeWheel) add(node *timeNode, jiffies uint64) error {
	if node == nil {
		return ErrNilNode
	}

	var head *Time
	expire := node.expire

	// Prevent negative overflow when calculating remaining ticks
	var idx uint64
	if expire > jiffies {
		idx = expire - jiffies
	} else {
		idx = 0
	}

	level, index := uint64(1), uint64(0)

	if idx < nearSize {
		// Timer expires within the near term (< 256 ticks)
		// Add to t1 (near list)
		index = uint64(expire) & nearMask
		// Boundary check (nearMask ensures index < nearSize, but double-check for safety)
		if index >= nearSize {
			return newError("add", ErrIndexOutOfRange, "t1 index out of range")
		}
		head = t.t1[index]
	} else {
		// Timer expires in the future - find appropriate tier
		val := t.maxVal()
		for i := 0; i <= 3; i++ {
			// Cap idx at maximum value
			if idx > val {
				idx = val
				expire = idx + jiffies
			}

			// Check if this tier can handle the expiration time
			if idx < t.levelMax(i+1) {
				// Calculate slot index for this tier
				index = uint64(expire >> (nearShift + i*levelShift) & levelMask)
				// Boundary check (levelMask ensures index < levelSize)
				if index >= levelSize {
					return newError("add", ErrIndexOutOfRange, "t2tot5 slot index out of range")
				}
				// i is in range [0,3], no need to check bounds
				head = t.t2Tot5[i][index]
				level = uint64(i) + 2
				break
			}
		}
	}

	if head == nil {
		return newError("add", ErrHeadNotFound, "no appropriate head found for the node")
	}

	return head.lockPushBack(node, level, index)
}

// AfterFunc schedules a one-shot callback to be executed after the specified duration.
// The callback is executed in a separate goroutine when the timer expires.
//
// Parameters:
//
//	expire: duration until callback execution
//	callback: function to call when timer expires
//
// Returns:
//
//	TimeNoder interface for controlling the timer and error
func (t *timeWheel) AfterFunc(expire time.Duration, callback func()) (TimeNoder, error) {
	if t == nil {
		return nil, ErrNilTimer
	}

	if expire <= 0 {
		return nil, newError("AfterFunc", ErrInvalidExpire, "expire must be greater than 0")
	}

	if callback == nil {
		return nil, ErrNilCallback
	}

	jiffies := atomic.LoadUint64(&t.jiffies)
	expireTicks := expire/timeUnit + time.Duration(jiffies)

	node := &timeNode{
		expire:   uint64(expireTicks),
		callback: callback,
		root:     t,
	}

	if err := t.add(node, jiffies); err != nil {
		return nil, newError("AfterFunc", err, "failed to add node")
	}

	return node, nil
}

// getExpire calculates the absolute expiration time in ticks.
// If expire <= 0, it defaults to one timeUnit.
// Returns: absolute tick count when timer should expire
func getExpire(expire time.Duration, jiffies uint64) time.Duration {
	if expire <= 0 {
		expire = timeUnit
	}
	return expire/timeUnit + time.Duration(jiffies)
}

// ScheduleFunc schedules a periodic callback that repeats at the specified interval.
// The callback is executed in a separate goroutine each time the timer expires.
// After execution, the timer is automatically rescheduled.
//
// Parameters:
//
//	userExpire: interval between callback executions
//	callback: function to call periodically
//
// Returns:
//
//	TimeNoder interface for controlling the timer and error
func (t *timeWheel) ScheduleFunc(userExpire time.Duration, callback func()) (TimeNoder, error) {
	if t == nil {
		return nil, ErrNilTimer
	}

	if userExpire <= 0 {
		return nil, newError("ScheduleFunc", ErrInvalidExpire, "userExpire must be greater than 0")
	}

	if callback == nil {
		return nil, ErrNilCallback
	}

	jiffies := atomic.LoadUint64(&t.jiffies)
	expire := getExpire(userExpire, jiffies)

	node := &timeNode{
		userExpire: userExpire,
		expire:     uint64(expire),
		callback:   callback,
		isSchedule: true, // Mark as periodic timer
		root:       t,
	}

	if err := t.add(node, jiffies); err != nil {
		return nil, newError("ScheduleFunc", err, "failed to add node")
	}

	return node, nil
}

// Stop cancels the timer and stops the main loop.
// It calls the context cancel function to signal shutdown.
func (t *timeWheel) Stop() {
	if t != nil && t.cancel != nil {
		t.cancel()
	}
}

// cascade moves timers from a higher tier down to lower tiers.
// This is called when a slot in a tier wraps around to 0.
// All timers in that slot are re-evaluated and placed in appropriate lower-tier slots.
//
// The cascade process:
// 1. Lock the source list and move all nodes to a temporary list
// 2. Increment the list's version number (invalidates node versions)
// 3. Unlock the source list
// 4. Re-add each node to the timewheel (they'll go to lower tiers)
//
// Parameters:
//
//	levelIndex: which tier to cascade from (0-3 for t2-t5)
//	index: which slot in that tier
//
// Returns: error if cascade fails
func (t *timeWheel) cascade(levelIndex int, index int) error {
	// Boundary check - levelIndex should be in [0,3] for t2-t5
	if levelIndex < 0 || levelIndex >= 4 {
		return newError("cascade", ErrIndexOutOfRange, "levelIndex out of range [0, 3]")
	}
	// index should be in [0, levelSize-1]
	if index < 0 || index >= levelSize {
		return newError("cascade", ErrIndexOutOfRange, "index out of range")
	}

	// Create temporary list to hold nodes during cascade
	tmp := newTimeHead(0, 0)
	l := t.t2Tot5[levelIndex][index]

	if l == nil {
		return newError("cascade", ErrNilTimer, "list head is nil")
	}

	l.Lock()
	if l.Len() == 0 {
		l.Unlock()
		return nil
	}

	// Move all nodes from source list to temporary list
	l.ReplaceInit(&tmp.Head)

	// Increment version to invalidate all node versions
	// This signals to Stop() that nodes have been moved
	currentVersion := l.version.Load()
	seq := (currentVersion & 0xFFFFFFFF) + 1
	l.version.Store((currentVersion & 0xFFFFFFFF00000000) | seq)
	l.Unlock()

	// Re-add each node to the timewheel
	offset := unsafe.Offsetof(tmp.Head)
	var lastErr error

	tmp.ForEachSafe(func(pos *list.Head) {
		if pos == nil {
			return
		}

		node := (*timeNode)(pos.Entry(offset))
		if node != nil {
			if err := t.add(node, atomic.LoadUint64(&t.jiffies)); err != nil {
				lastErr = err
			}
		}
	})

	return lastErr
}

// moveAndExec performs the core timewheel tick operation.
// It consists of three phases:
// 1. Cascade: If current slot is 0, cascade from higher tiers
// 2. Increment: Advance jiffies by 1
// 3. Execute: Run all callbacks in the current near-list slot
//
// For periodic timers, they are automatically rescheduled after execution.
//
// Returns: error if any phase fails
func (t *timeWheel) moveAndExec() error {
	// Phase 1: Cascade from higher tiers if needed
	// When a tier's slot wraps to 0, cascade from the next tier
	index := t.jiffies & nearMask
	if index == 0 {
		for i := 0; i <= 3; i++ {
			index2, err := t.index(i)
			if err != nil {
				return err
			}

			if err = t.cascade(i, int(index2)); err != nil {
				// Log error but continue - don't let cascade failure stop execution
			}

			// Only cascade one tier if its index is non-zero
			if index2 != 0 {
				break
			}
		}
	}

	// Phase 2: Increment jiffies (advance time by one tick)
	atomic.AddUint64(&t.jiffies, 1)

	// Phase 3: Execute callbacks in current near-list slot
	// Boundary check (nearMask ensures index < nearSize)
	if index >= nearSize {
		return newError("moveAndExec", ErrIndexOutOfRange, "index out of range for t1")
	}

	t1 := t.t1[index]
	if t1 == nil {
		return newError("moveAndExec", ErrNilTimer, "t1 head is nil")
	}

	t1.Lock()
	if t1.Len() == 0 {
		t1.Unlock()
		return nil
	}

	// Move all nodes to temporary list for execution
	head := newTimeHead(0, 0)
	t1.ReplaceInit(&head.Head)

	// Update version number (invalidates node versions)
	currentVersion := t1.version.Load()
	seq := (currentVersion & 0xFFFFFFFF) + 1
	t1.version.Store((currentVersion & 0xFFFFFFFF00000000) | seq)
	t1.Unlock()

	// Execute callbacks for each timer in the list
	offset := unsafe.Offsetof(head.Head)

	head.ForEachSafe(func(pos *list.Head) {
		if pos == nil {
			return
		}

		val := (*timeNode)(pos.Entry(offset))
		if val == nil {
			return
		}

		head.Del(pos)

		// Skip if node was stopped
		if val.stop.Load() == haveStop {
			return
		}

		// Execute callback in separate goroutine
		if val.callback != nil {
			go val.callback()
		}

		// Reschedule periodic timers
		if val.isSchedule {
			jiffies := t.jiffies
			// IMPORTANT: Subtract 1 from jiffies to account for the current tick
			// The callback has already consumed one tick, so we need to subtract it
			// to maintain accurate periodic timing. Otherwise, each period would
			// accumulate an extra tick, causing the timer to drift.
			if jiffies > 0 {
				val.expire = uint64(getExpire(val.userExpire, jiffies-1))
			} else {
				val.expire = uint64(getExpire(val.userExpire, 0))
			}

			// Re-add to timewheel (errors are ignored to not interrupt execution)
			_ = t.add(val, jiffies)
		}
	})

	return nil
}

// run processes elapsed time and executes expired timers.
// It's parameterized with a time function for testability.
//
// The algorithm:
// 1. Get current time point
// 2. Check for time going backwards
// 3. Calculate elapsed time since last run
// 4. Execute moveAndExec for each elapsed tick (up to maxDiff limit)
//
// Parameters:
//
//	get3Ms: function to get current time in 3ms units
//
// Returns: error if time goes backwards or execution fails
func (t *timeWheel) exec() error {
	// Get current time in 3ms units
	ms3 := t.get3Ms()

	// Check for time going backwards (clock adjustment)
	if ms3 < t.curTimePoint {
		return newError("run", ErrTimeGoBack, "time has gone backwards")
	}

	// Calculate elapsed time
	diff := ms3 - t.curTimePoint
	t.curTimePoint = ms3

	// Limit processing to prevent long blocking
	// Maximum 1000 ticks (3 seconds) per run
	maxDiff := int64(1000)
	if int64(diff) > maxDiff {
		diff = time.Duration(maxDiff)
	}

	// Process each elapsed tick
	for i := 0; i < int(diff); i++ {
		if err := t.moveAndExec(); err != nil {
			return err
		}
	}

	return nil
}

// run starts the timer's main loop.
// It ticks every 3ms (timeUnit) and processes expired timers.
// The loop runs until the context is cancelled or an error occurs.
func (t *timeWheel) run() {
	t.curTimePoint = t.get3Ms()
	// Create ticker for 3ms intervals
	tk := time.NewTicker(timeUnit)
	defer tk.Stop()

	for {
		select {
		case <-tk.C:
			// Process elapsed time on each tick
			if err := t.exec(); err != nil {
				slog.Error(fmt.Sprintf("%v", err))
			}
		case <-t.ctx.Done():
			// Context cancelled - clean shutdown
			slog.Error(fmt.Sprintf("%v", t.ctx.Err()))
			return
		}
	}
}

// get3Ms returns the current time in 3ms units.
// It converts nanoseconds to timeUnit (3ms) intervals.
// Returns 0 if time is negative or timeUnit is 0.
func (t *timeWheel) get3Ms() time.Duration {
	now := time.Now().UnixNano()
	if now < 0 {
		return 0
	}
	unit := int64(timeUnit)
	if unit == 0 {
		return 0
	}
	return time.Duration(now / unit)
}
