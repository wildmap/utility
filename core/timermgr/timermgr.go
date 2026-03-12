package timermgr

import (
	"fmt"

	"github.com/wildmap/utility/xlog"
	"github.com/wildmap/utility/xtime"
)

// AccKind 定时器时间调整的计算方式枚举。
type AccKind int32

const (
	// AccAbs 按绝对毫秒值调整，value 为正整数毫秒数。
	AccAbs AccKind = iota
	// AccPct 按万分比调整，value 范围 [1, 10000]，10000 表示 100%。
	AccPct
)

const (
	// PctBase AccPct 模式的基数（万分之一），用于将万分比换算为实际时长缩放比例。
	// 选用 10000 而非 100 是为了支持小数百分比（如 0.5% = value 50），提高调整精度。
	PctBase = 10000
)

// TimerHandler 定时器触发回调函数类型。
//
// timerID 用于在回调中反向查询定时器元数据，
// metadata 为创建定时器时附加的业务数据（如关联的实体 ID、任务类型等）。
type TimerHandler func(timerID int64, metadata map[string]string)

// Timer 业务层定时器的完整元数据，与底层 Dispatcher 的定时器节点一一对应。
//
// 分层设计的意义：业务层（TimerMgr + Timer）负责定时器的语义，包括类型路由、
// 元数据管理和 Ticker 的自动续期；底层 Dispatcher 只关注时间轮调度，
// 两层通过 timerID 解耦，各司其职，便于独立测试和替换。
type Timer struct {
	id       int64             // 定时器唯一 ID，与 Dispatcher 中的定时器节点共享同一 ID
	kind     string            // 定时器类型标识，用于路由到对应的 TimerHandler
	startTs  int64             // 定时器本次周期的起始时间戳（毫秒），Ticker 续期时用作计算下次到期时间的基准
	endTs    int64             // 定时器期望触发的绝对时间戳（毫秒）
	isTicker bool              // true 表示周期性 Ticker，触发后自动续期；false 表示一次性 Timer
	metadata map[string]string // 业务元数据，触发时原样透传给 TimerHandler，不由框架解析
}

// GetID 返回定时器 ID。
func (t *Timer) GetID() int64 {
	return t.id
}

// GetKind 返回定时器类型标识。
func (t *Timer) GetKind() string {
	return t.kind
}

// GetStartTs 返回定时器本次周期的起始时间戳（毫秒）。
func (t *Timer) GetStartTs() int64 {
	return t.startTs
}

// GetEndTs 返回定时器的期望触发时间戳（毫秒）。
func (t *Timer) GetEndTs() int64 {
	return t.endTs
}

// IsTicker 返回是否为周期性定时器。
func (t *Timer) IsTicker() bool {
	return t.isTicker
}

// RangeMetadata 遍历定时器的所有元数据键值对，回调返回 false 时提前终止遍历。
func (t *Timer) RangeMetadata(f func(string, string) bool) {
	for k, v := range t.metadata {
		if !f(k, v) {
			break
		}
	}
}

// TimerMgr 业务层定时器管理器，在 Skeleton 的事件循环中负责定时器的创建、续期和销毁。
//
// 并发安全说明：TimerMgr 的所有方法均在模块的单一 goroutine（Skeleton.OnStart 事件循环）中执行，
// 无需加锁。底层 Dispatcher 的并发安全由其 chanOp 通道串行化保证。
//
// timers 存储业务层元数据，通过 timerID 与 Dispatcher 中的时间轮节点关联。
// handlers 按 kind 存储回调函数，NewTimer/NewTicker 时校验 kind 是否已注册。
type TimerMgr struct {
	timers     map[int64]*Timer        // timerID → 定时器业务元数据
	handlers   map[string]TimerHandler // kind → 处理函数，注册后不再修改
	dispatcher *Dispatcher             // 底层多级时间轮分发器，在独立 goroutine 中运行
}

// NewTimerMgr 创建定时器管理器，参数 l 为底层分发器通道的缓冲容量。
func NewTimerMgr(l int) *TimerMgr {
	return &TimerMgr{
		timers:     make(map[int64]*Timer),
		handlers:   make(map[string]TimerHandler),
		dispatcher: NewDispatcher(l),
	}
}

// RegisterTimer 注册指定 kind 类型的定时器回调函数。
//
// 同 kind 后注册的处理器会覆盖前者，业务层需保证 kind 全局唯一，
// 通常在模块 OnInit 阶段完成注册，之后不再修改。
func (tm *TimerMgr) RegisterTimer(kind string, handler TimerHandler) {
	tm.handlers[kind] = handler
}

// Run 启动底层时间轮分发器的后台 goroutine，必须在创建任何定时器之前调用。
func (tm *TimerMgr) Run() {
	tm.dispatcher.Run()
}

// Stop 停止底层时间轮分发器，所有未触发的定时器将被丢弃。
func (tm *TimerMgr) Stop() {
	tm.dispatcher.Stop()
}

// ChanTimer 返回定时器触发通道，供 Skeleton 的事件循环监听定时器到期事件。
func (tm *TimerMgr) ChanTimer() <-chan *dispatcherTimer {
	return tm.dispatcher.ChanTimer
}

// GetTimer 通过 ID 查询定时器业务元数据，未找到时返回 nil。
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

// GetTimerByKind 通过 kind 查询第一个匹配的定时器元数据。
//
// 适用于全局唯一单例定时器的查找（如"每日重置定时器"）；
// 若同 kind 存在多个定时器，仅返回遍历中首个命中的，结果不确定，业务层需自行保证唯一性。
func (tm *TimerMgr) GetTimerByKind(kind string) *Timer {
	for _, timer := range tm.timers {
		if timer.kind == kind {
			return timer
		}
	}
	return nil
}

// timerCommonCb 定时器统一触发入口，由 Dispatcher 在到期时调用。
//
// Ticker 续期算法（防漂移设计）：
//   - 以上次到期时间（oldEndTs）而非当前时间（now）作为新一轮的起始点
//   - 新的 endTs = oldEndTs + (oldEndTs - startTs)，即保持与创建时相同的间隔
//   - 这样即使回调处理耗时较长，下次到期时间也不会因处理延迟而发生累积漂移
//
// 一次性定时器触发后自动调用 CancelTimer 清理业务层元数据，防止内存泄漏。
func (tm *TimerMgr) timerCommonCb(timerID int64) {
	t := tm.getTimer(timerID)
	if t == nil {
		xlog.Errorf("delay timer timerID %v not found", timerID)
		return
	}
	if xtime.NowTs() < t.endTs {
		xlog.Errorf("delay timer timerCommonCb timer endTs bigger than nowMs")
	}
	f, ok := tm.handlers[t.kind]
	if !ok {
		xlog.Errorf("delay timer timer kind %s not found", t.kind)
		return
	}
	defer func() {
		if t.isTicker {
			// 以上次到期时间为基准计算下次触发时间，保证周期稳定不漂移
			oldEndTs := t.endTs
			t.endTs += t.endTs - t.startTs // 新 endTs = oldEndTs + 周期长度
			t.startTs = oldEndTs           // 更新 startTs 为本次到期时间，为下次续期做准备
			tm.dispatcher.NewTimer(timerID, t.endTs, tm.timerCommonCb)
		} else {
			// 一次性定时器触发后自动清理，防止元数据泄漏
			tm.CancelTimer(timerID)
		}
	}()
	f(timerID, t.metadata)
}

// newTimer 创建定时器的内部实现，通过 isTicker 参数统一处理一次性和周期性两种情况。
//
// 创建流程：校验 kind → 计算到期时间 → 注册到 Dispatcher → 存储业务元数据。
// id 为 0 时由 Dispatcher 自动生成全局唯一 ID（通过 utility.NextID）。
func (tm *TimerMgr) newTimer(id int64, duraMs int64, kind string, metadata map[string]string, isTicker bool) int64 {
	_, ok := tm.handlers[kind]
	if !ok {
		xlog.Errorf("TimerMgr NewTimer timer kind %s not found", kind)
		return 0
	}
	startTs := xtime.NowTs()
	endTs := startTs + duraMs
	id = tm.dispatcher.NewTimer(id, endTs, tm.timerCommonCb)
	tm.setTimer(id, &Timer{
		id:       id,
		kind:     kind,
		startTs:  startTs,
		endTs:    endTs,
		metadata: metadata,
		isTicker: isTicker,
	})
	return id
}

// NewTimer 创建一次性定时器，在 duraMs 毫秒后触发一次，自动生成唯一 ID。
func (tm *TimerMgr) NewTimer(duraMs int64, kind string, metadata map[string]string) int64 {
	return tm.newTimer(0, duraMs, kind, metadata, false)
}

// NewTicker 创建周期性定时器，每隔 duraMs 毫秒触发一次，并自动续期直到被取消。
//
// id 为 0 时自动生成新 ID；传入已有 ID 时会覆盖（更新）该 Ticker 的周期和元数据，
// 可用于运行时动态调整已有 Ticker 的触发间隔，无需先取消再创建。
func (tm *TimerMgr) NewTicker(id int64, duraMs int64, kind string, metadata map[string]string) int64 {
	return tm.newTimer(id, duraMs, kind, metadata, true)
}

// AccTimer 加速定时器，提前其触发时间。
//
// AccAbs 模式：新剩余时间 = max(0, 原剩余时间 - value)，value 必须为正整数（毫秒）。
// AccPct 模式：新剩余时间 = 原剩余时间 × (PctBase - value) / PctBase，value ∈ [1, PctBase]。
// 两种模式下新剩余时间均不低于 0，防止触发时间被推到过去引发立即触发的不预期行为。
func (tm *TimerMgr) AccTimer(id int64, kind AccKind, value int64) error {
	nowTs := xtime.NowTs()
	t := tm.getTimer(id)
	if t == nil {
		return fmt.Errorf("acc timer failed, timer %v not found", id)
	}
	remain := t.endTs - nowTs
	newRemain := int64(0)
	switch kind {
	case AccAbs:
		if value <= 0 {
			return fmt.Errorf("acc timer failed, invalid args: %d %d %d", id, kind, value)
		}
		newRemain = max(0, remain-value) // 加速后不低于 0，防止触发时间回退到过去
	case AccPct:
		if value <= 0 || value > PctBase {
			return fmt.Errorf("acc timer failed, invalid args: %d %d %d", id, kind, value)
		}
		newRemain = remain * (PctBase - value) / PctBase
	default:
		return fmt.Errorf("acc timer failed, invalid args: %d %d %d", id, kind, value)
	}
	newEndTs := nowTs + newRemain
	t.endTs = newEndTs
	tm.dispatcher.UpdateTimer(id, newEndTs)

	return nil
}

// DelayTimer 延迟定时器，推迟其触发时间。
//
// AccAbs 模式：新剩余时间 = 原剩余时间 + value，value 必须为正整数（毫秒）。
// AccPct 模式：新剩余时间 = 原剩余时间 × (PctBase + value) / PctBase，value ∈ [1, PctBase]。
// 注意：对已到期但尚未被事件循环消费的定时器调用延迟可能无效，因为 Dispatcher 已将其投递到触发通道。
func (tm *TimerMgr) DelayTimer(id int64, kind AccKind, value int64) (err error) {
	nowTs := xtime.NowTs()
	t := tm.getTimer(id)
	if t == nil {
		return fmt.Errorf("delay timer failed, timer %v not found", id)
	}
	remain := t.endTs - nowTs
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
	t.endTs = newEndTs
	tm.dispatcher.UpdateTimer(id, newEndTs)

	return
}

// UpdateTimer 直接将定时器的到期时间更新为指定的绝对时间戳（毫秒）。
//
// 与 AccTimer/DelayTimer 不同，此方法接受绝对时间戳而非相对偏移量，
// 适合需要精确指定到期时刻的场景（如同步到服务器的绝对时间点）。
func (tm *TimerMgr) UpdateTimer(id int64, endTs int64) {
	tm.dispatcher.UpdateTimer(id, endTs)
}

// CancelTimer 取消定时器并同步清理业务层元数据。
//
// id 为 0 时记录错误日志并直接返回，因为 id=0 是 Dispatcher 的内置停止信号，
// 业务层绝不应使用 id=0 的定时器，防止误操作导致分发器意外停止。
// 清理 timers 中的元数据与底层取消是分开进行的，
// 底层通过双重机制（canceledTimers + chanOp）保证取消可靠，业务层只需删除本地元数据。
func (tm *TimerMgr) CancelTimer(id int64) {
	if id == 0 {
		xlog.Errorf("TimerMgr CancelTimer timerID = 0")
		return
	}
	tm.dispatcher.CancelTimer(id)
	delete(tm.timers, id) // 同步清理业务层元数据，防止 timers map 无限增长
}
