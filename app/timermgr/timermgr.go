package timermgr

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/wildmap/utility"
)

// AccKind Timer 加速类型
type AccKind int32

const (
	// AccAbs 按绝对值加速 必须>0
	AccAbs AccKind = iota
	// AccPct 按百分比加速 万分比 [0, 10000]
	AccPct
)

const (
	// PctBase AccPct 基于万分比
	PctBase = 10000
)

type TimerHandler func(int64, map[string]string)

type Timer struct {
	ID       int64             // ID
	Kind     string            // 类型
	StartTs  int64             // 开始时间
	EndTs    int64             // 结束时间
	IsTicker bool              // 是否为定时任务
	Metadata map[string]string // 元数据
}

type TimerMgr struct {
	sync.Mutex
	Timers     map[int64]*Timer
	handlers   map[string]TimerHandler
	dispatcher *Dispatcher
}

func NewTimerMgr() *TimerMgr {
	return &TimerMgr{
		Timers:     make(map[int64]*Timer),
		handlers:   make(map[string]TimerHandler),
		dispatcher: NewDispatcher(),
	}
}

func (tm *TimerMgr) Run() {
	tm.dispatcher.Run()
}

func (tm *TimerMgr) Stop() {
	tm.dispatcher.Stop()
}

func (tm *TimerMgr) ChanTimer() <-chan *dispatcherTimer {
	return tm.dispatcher.ChanTimer
}

func (tm *TimerMgr) timerCommonCb(timerID int64) {
	tm.Lock()
	defer tm.Unlock()

	t := tm.Timers[timerID]
	if t == nil {
		return
	}
	if utility.NowTs() < t.EndTs {
		slog.Error(fmt.Sprintf("delay timer timerCommonCb timer endTs bigger than nowMs"))
	}
	f := tm.handlers[t.Kind]
	defer func() {
		if t.IsTicker {
			oldEndTs := t.EndTs
			t.EndTs += t.EndTs - t.StartTs
			t.StartTs = oldEndTs
			tm.dispatcher.NewTimer(timerID, t.EndTs, tm.timerCommonCb)
		} else {
			delete(tm.Timers, timerID)
		}
	}()
	f(timerID, t.Metadata)
}

func (tm *TimerMgr) GetTimer(timerID int64) *Timer {
	tm.Lock()
	defer tm.Unlock()

	return tm.Timers[timerID]
}

func (tm *TimerMgr) GetTimerByKind(kind string) *Timer {
	tm.Lock()
	defer tm.Unlock()

	for _, timer := range tm.Timers {
		if timer.Kind == kind {
			return timer
		}
	}

	return nil
}

func (tm *TimerMgr) newTimer(duraMs int64, kind string, metadata map[string]string, isTicker bool) int64 {
	tm.Lock()
	defer tm.Unlock()

	_, ok := tm.handlers[kind]
	if !ok {
		return 0
	}
	startTs := utility.NowTs()
	endTs := startTs + duraMs
	var id = tm.dispatcher.NewTimer(0, startTs+duraMs, tm.timerCommonCb)
	timer := &Timer{
		ID:       id,
		Kind:     kind,
		StartTs:  startTs,
		EndTs:    endTs,
		Metadata: metadata,
		IsTicker: isTicker,
	}
	tm.Timers[id] = timer
	return id
}

func (tm *TimerMgr) NewTimer(duraMs int64, kind string, metadata map[string]string) int64 {
	return tm.newTimer(duraMs, kind, metadata, false)
}

func (tm *TimerMgr) NewTicker(duraMs int64, kind string, metadata map[string]string) int64 {
	return tm.newTimer(duraMs, kind, metadata, true)
}

func (tm *TimerMgr) AccTimer(id int64, kind AccKind, value int64) error {
	tm.Lock()
	defer tm.Unlock()

	nowTs := utility.NowTs()
	t := tm.Timers[id]
	if t == nil {
		return fmt.Errorf("acc timer failed, timer %v not found", id)
	}
	remain := t.EndTs - nowTs
	newRemain := int64(0)
	switch kind {
	case AccAbs:
		if value <= 0 {
			return fmt.Errorf("acc timer failed, invalid args: %d %d %d", id, kind, value)
		}
		newRemain = max(0, remain-value)
	case AccPct:
		if value <= 0 || value > PctBase {
			return fmt.Errorf("acc timer failed, invalid args: %d %d %d", id, kind, value)
		}
		newRemain = remain * (PctBase - value) / PctBase
	default:
		return fmt.Errorf("acc timer failed, invalid args: %d %d %d", id, kind, value)
	}
	newEndTs := nowTs + newRemain
	t.EndTs = newEndTs
	tm.dispatcher.UpdateTimer(id, newEndTs)

	return nil
}

// DelayTimer 延迟Timer
func (tm *TimerMgr) DelayTimer(id int64, kind AccKind, value int64) (err error) {
	tm.Lock()
	defer tm.Unlock()

	nowTs := utility.NowTs()
	t := tm.Timers[id]
	if t == nil {
		return fmt.Errorf("delay timer failed, timer %v not found", id)
	}
	remain := t.EndTs - nowTs
	newRemain := int64(0)
	switch kind {
	case AccAbs:
		if value <= 0 {
			return fmt.Errorf("delay timer failed, invalid args: %d %d %d", id, kind, value)
		}
		newRemain = remain + value
	case AccPct:
		if value <= 0 || value > PctBase {
			return fmt.Errorf("delay timer failed, invalid args: %d %d %d", id, kind, value)
		}
		newRemain = remain * (PctBase + value) / PctBase
	default:
		return fmt.Errorf("delay timer failed, invalid args: %d %d %d", id, kind, value)
	}
	newEndTs := nowTs + newRemain
	t.EndTs = newEndTs
	tm.dispatcher.UpdateTimer(id, newEndTs)

	return
}

// CancelTimer 取消一个定时器
func (tm *TimerMgr) CancelTimer(id int64) {
	tm.Lock()
	defer tm.Unlock()

	if id == 0 {
		slog.Error(fmt.Sprintf("TimerMgr CancelTimer timerID = 0"))
		return
	}
	tm.dispatcher.CancelTimer(id)
	delete(tm.Timers, id)
}

// RegisterTimer 注册指定类型timer处理函数
func (tm *TimerMgr) RegisterTimer(kind string, handler TimerHandler) {
	tm.Lock()
	defer tm.Unlock()

	tm.handlers[kind] = handler
}
