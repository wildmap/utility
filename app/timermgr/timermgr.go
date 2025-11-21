package timermgr

import (
	"fmt"

	"github.com/wildmap/utility/xlog"
	"github.com/wildmap/utility/xtime"
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
	timers     map[int64]*Timer
	handlers   map[string]TimerHandler
	dispatcher *Dispatcher
}

func NewTimerMgr(l int) *TimerMgr {
	return &TimerMgr{
		timers:     make(map[int64]*Timer),
		handlers:   make(map[string]TimerHandler),
		dispatcher: NewDispatcher(l),
	}
}

// RegisterTimer 注册指定类型timer处理函数
func (tm *TimerMgr) RegisterTimer(kind string, handler TimerHandler) {
	tm.handlers[kind] = handler
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

func (tm *TimerMgr) GetTimer(timerID int64) *Timer {
	return tm.getTimer(timerID)
}

func (tm *TimerMgr) getTimer(timerID int64) *Timer {
	t := tm.timers[timerID]
	return t
}

func (tm *TimerMgr) setTimer(timerID int64, timer *Timer) {
	tm.timers[timerID] = timer
}

func (tm *TimerMgr) GetTimerByKind(kind string) *Timer {
	for _, timer := range tm.timers {
		if timer.Kind == kind {
			return timer
		}
	}
	return nil
}

func (tm *TimerMgr) timerCommonCb(timerID int64) {
	t := tm.getTimer(timerID)
	if t == nil {
		xlog.Errorf("delay timer timerID %v not found", timerID)
		return
	}
	if xtime.NowTs() < t.EndTs {
		xlog.Errorf("delay timer timerCommonCb timer endTs bigger than nowMs")
	}
	f, ok := tm.handlers[t.Kind]
	if !ok {
		xlog.Errorf("delay timer timer kind %s not found", t.Kind)
		return
	}
	defer func() {
		if t.IsTicker {
			oldEndTs := t.EndTs
			t.EndTs += t.EndTs - t.StartTs
			t.StartTs = oldEndTs
			tm.dispatcher.NewTimer(timerID, t.EndTs, tm.timerCommonCb)
		} else {
			tm.CancelTimer(timerID)
		}
	}()
	f(timerID, t.Metadata)
}

func (tm *TimerMgr) newTimer(duraMs int64, kind string, metadata map[string]string, isTicker bool) int64 {
	_, ok := tm.handlers[kind]
	if !ok {
		xlog.Errorf("TimerMgr NewTimer timer kind %s not found", kind)
		return 0
	}
	startTs := xtime.NowTs()
	endTs := startTs + duraMs
	var id = tm.dispatcher.NewTimer(0, endTs, tm.timerCommonCb)
	tm.setTimer(id, &Timer{
		ID:       id,
		Kind:     kind,
		StartTs:  startTs,
		EndTs:    endTs,
		Metadata: metadata,
		IsTicker: isTicker,
	})
	return id
}

func (tm *TimerMgr) NewTimer(duraMs int64, kind string, metadata map[string]string) int64 {
	return tm.newTimer(duraMs, kind, metadata, false)
}

func (tm *TimerMgr) NewTicker(duraMs int64, kind string, metadata map[string]string) int64 {
	return tm.newTimer(duraMs, kind, metadata, true)
}

func (tm *TimerMgr) AccTimer(id int64, kind AccKind, value int64) error {
	nowTs := xtime.NowTs()
	t := tm.getTimer(id)
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
	nowTs := xtime.NowTs()
	t := tm.getTimer(id)
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
	if id == 0 {
		xlog.Errorf("TimerMgr CancelTimer timerID = 0")
		return
	}
	tm.dispatcher.CancelTimer(id)
	delete(tm.timers, id)
}
