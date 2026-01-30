package timermgr

/* timer 将Timer跑在单独的goroutine中
 * 以精度换取效率
 */

import (
	"runtime/debug"
	"sync"
	"time"

	"github.com/wildmap/utility"
	"github.com/wildmap/utility/xlog"
	"github.com/wildmap/utility/xtime"
)

// Timer 默认定义
const (
	timerTick  = 4  // 最小粒度 ms 考虑将其定义为2^N 提高效率 (4=2^2)
	timerLevel = 28 // 时间段最大分级，支持的最大时间段为 2^TIMERLEVEL*timerTick (2^28*4ms≈12.4天)
)

// Dispatcher 定时器分发
type Dispatcher struct {
	timerSlots     [timerLevel]map[int64]*dispatcherTimer // 时间轮
	chanOp         chan *dispatcherTimer                  // 用于向Dispather发送Timer相关操作命令
	ChanTimer      chan *dispatcherTimer                  // Dispather中Timer到了，用于通知使用者
	canceledTimers sync.Map                               // 已取消的定时器ID集合，用于快速忽略后续操作
}

type dispatcherTimer struct {
	id    int64       // ID
	endTs int64       // 到期时间戳 ms
	cb    func(int64) // 回调
}

// Cb 执行tiemr回调
func (t dispatcherTimer) Cb() {
	defer func() {
		t.cb = nil
		if r := recover(); r != nil {
			xlog.Errorf("%v\n%s", r, string(debug.Stack()))
		}
	}()
	t.cb(t.id)
}

// NewDispatcher 时间轮分发器
func NewDispatcher(l int) *Dispatcher {
	disp := new(Dispatcher)
	for k := range disp.timerSlots {
		disp.timerSlots[k] = make(map[int64]*dispatcherTimer)
	}
	if l <= 0 {
		l = 10000
	}

	disp.chanOp = make(chan *dispatcherTimer, l)
	disp.ChanTimer = make(chan *dispatcherTimer, l)

	return disp
}

// Run 运行分发器
func (disp *Dispatcher) Run() {
	go disp.run()
}

func (disp *Dispatcher) run() {
	defer func() {
		if x := recover(); x != nil {
			xlog.Errorf("TIMER CRASHED %v\n%s", x, string(debug.Stack()))
		}
	}()

	lastTick := xtime.Now().UnixMilli() / timerTick
	tickTimer := time.NewTimer(timerTick * time.Millisecond)
	for {
		select {
		case t := <-disp.chanOp:
			if !disp.doOp(t) {
				return
			}
		case <-tickTimer.C:
			tickTimer.Reset(timerTick * time.Millisecond)
			lastTick = disp.doTick(xtime.Now(), lastTick)
		}
	}
}

// 执行对应的操作
// 为了复用结构体，用timer结构来区分不同的操作:
// timer.endTs == 0: 取消Timer
// timer.endTs != 0 && timer.cb != nil: 新建Timer
// timer.endTs != 0 && timer.cb == nil: 更新Timer
func (disp *Dispatcher) doOp(t *dispatcherTimer) bool {
	// 操作A: 删除Timer（优先处理取消操作）
	if t.endTs == 0 {
		disp.canceledTimers.Store(t.id, struct{}{}) // 标记为已取消
		disp.delete(t.id)
		return true
	}

	// 检查定时器是否已被取消，如果已取消则忽略后续操作
	if _, canceled := disp.canceledTimers.Load(t.id); canceled {
		return true
	}

	// 操作B: 停止Dispathcer
	if t.id == 0 {
		return false
	}

	// 操作C: 创建Timer
	if t.endTs != 0 && t.cb != nil {
		disp.canceledTimers.Delete(t.id) // 清除旧的取消标记（如果存在）
		disp.place(t)
		return true
	}

	// 操作D: 更新Timer
	if t.endTs != 0 && t.cb == nil {
		// 找到并删除Timer
		oldt := disp.delete(t.id)
		// 重新找合适的框
		if oldt != nil {
			oldt.endTs = t.endTs
			disp.place(oldt)
		} else {
			xlog.Errorf("delay timer%d, get old timer fail", t.id)
		}
		return true
	}

	return true
}

// 删除并返回Timer
func (disp *Dispatcher) delete(timerID int64) *dispatcherTimer {
	for i := timerLevel - 1; i >= 0; i-- {
		if v, ok := disp.timerSlots[i][timerID]; ok {
			delete(disp.timerSlots[i], timerID)
			disp.canceledTimers.Delete(timerID) // 清除取消标记，避免内存泄漏
			return v
		}
	}
	disp.canceledTimers.Delete(timerID) // 即使没找到timer，也要清除取消标记
	return nil
}

// 将Timer放到合适的时间轮中
func (disp *Dispatcher) place(t *dispatcherTimer) {
	// 检查定时器是否已被取消
	if _, canceled := disp.canceledTimers.Load(t.id); canceled {
		return
	}

	diff := t.endTs - xtime.Now().UnixMilli()
	if diff <= 0 {
		select { // 发送必须为非阻塞, 传入的chan不能关闭
		case disp.ChanTimer <- t:
		default:
		}
		return
	}
	if diff < timerTick {
		diff = timerTick
	}
	// 从低层向高层遍历，找到合适的层级
	// 第i层存放剩余时间在 [2^(i-1)*timerTick, 2^i*timerTick) 区间的timer（i>0）
	// 第0层存放剩余时间 < timerTick 的timer
	for i := 0; i < timerLevel; i++ {
		if diff <= (timerTick << uint(i)) {
			disp.timerSlots[i][t.id] = t
			break
		}
	}

}

func (disp *Dispatcher) doTick(now time.Time, lastTick int64) int64 {
	// 防止服务器时间手动调整前移后 Timer重复触发
	nowMs := now.UnixMilli()
	nowTick := nowMs / timerTick
	if nowTick-lastTick < 1 {
		return nowTick
	}

	// 服务器时间手动调整后移后 Timer向前触发
	for {
		lastTick++
		for i := timerLevel - 1; i >= 0; i-- {
			mask := (1 << uint(i)) - 1
			if lastTick&int64(mask) == 0 {
				disp.trigger(nowMs, i)
			}
		}

		// 如果到达当前时间，停止循环
		if lastTick >= nowTick {
			break
		}
	}
	return nowTick
}

// 单级触发, 当level>0, 时间在[2^(level-1), 2^level)*TIMER_TICK区间内, 当level=0, 到期时间<timerTick
func (disp *Dispatcher) trigger(nowMs int64, level int) {
	slotMap := disp.timerSlots[level]
	for k, v := range slotMap {
		// 检查定时器是否已被取消
		if _, canceled := disp.canceledTimers.Load(k); canceled {
			delete(slotMap, k)
			disp.canceledTimers.Delete(k) // 清除取消标记
			continue
		}

		// 必须先计算移位，再乘以timerTick
		if v.endTs-nowMs < ((1 << uint(level)) * timerTick) {
			if level != 0 {
				disp.timerSlots[level-1][k] = v
				delete(slotMap, k)
			} else if nowMs >= v.endTs {
				select { // 发送必须为非阻塞, 传入的chan不能关闭
				case disp.ChanTimer <- v:
					delete(slotMap, k) // 如果发送失败，则尝试下次再次触发
				default:
				}
			}
		}
	}
}

// Stop 停止Dispatcher
func (disp *Dispatcher) Stop() {
	disp.chanOp <- &dispatcherTimer{id: 0, endTs: 0}
}

// UpdateTimer 加速 Timer
func (disp *Dispatcher) UpdateTimer(timerID, newEndTs int64) {
	disp.chanOp <- &dispatcherTimer{id: timerID, endTs: newEndTs}
}

// NewTimer 创建一个定时器
func (disp *Dispatcher) NewTimer(timerID, timeout int64, cb func(int64)) int64 {
	if timerID == 0 {
		timerID = utility.NextID().Int64()
	}
	disp.chanOp <- &dispatcherTimer{id: timerID, endTs: timeout, cb: cb}
	return timerID
}

// CancelTimer 取消定时器
func (disp *Dispatcher) CancelTimer(timerID int64) {
	disp.canceledTimers.Store(timerID, struct{}{}) // 立即标记为已取消
	disp.chanOp <- &dispatcherTimer{id: timerID, endTs: 0}
}
