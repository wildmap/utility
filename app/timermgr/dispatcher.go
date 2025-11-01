package timermgr

/* timer 将Timer跑在单独的goroutine中
 * 以精度换取效率
 * 此模块结合Skeleton使用
 */

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/wildmap/utility"
)

// Timer 默认定义
const (
	timerTick  = 64 // 最小粒度 ms 考虑将其定义为2^N 提高效率
	timerLevel = 20 // 时间段最大分级，支持的最大时间段为 2^TIMERLEVEL*timerTick
)

// Dispatcher 定时器分发
type Dispatcher struct {
	timerSlots [timerLevel]map[int64]*dispatcherTimer
	chanOp     chan *dispatcherTimer // 用于向Dispather发送Timer相关操作命令
	ChanTimer  chan *dispatcherTimer // Dispather中Timer到了，用于通知使用者
	chanSig    chan bool
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
			slog.Error(fmt.Sprintf("%v\n%s", r, string(debug.Stack())))
		}
	}()
	t.cb(t.id)
}

// NewDispatcher 时间轮分发器
func NewDispatcher() *Dispatcher {
	disp := new(Dispatcher)
	for k := range disp.timerSlots {
		disp.timerSlots[k] = make(map[int64]*dispatcherTimer)
	}

	disp.chanOp = make(chan *dispatcherTimer, 10000)
	disp.ChanTimer = make(chan *dispatcherTimer, 10000)
	disp.chanSig = make(chan bool, 1)

	return disp
}

// Run 运行分发器
func (disp *Dispatcher) Run() {
	go disp.run()
}

func (disp *Dispatcher) run() {
	defer func() {
		if x := recover(); x != nil {
			slog.Error(fmt.Sprintf("TIMER CRASHED %v\n%s", x, string(debug.Stack())))
		}
	}()

	lastTick := utility.ToUTC(utility.Now()).UnixNano() / 1e6 / timerTick
	tickTimer := time.NewTimer(timerTick * time.Millisecond)
	for {
		select {
		case t := <-disp.chanOp:
			if !disp.doOp(t) {
				return
			}
		case <-tickTimer.C:
			tickTimer.Reset(timerTick * time.Millisecond)
			lastTick = disp.doTick(utility.ToUTC(utility.Now()), lastTick)
		}
	}
}

// 执行对应的操作 返回是否停止Dispatcher
// 为了复用结构体，用timer结构来区分不同的操作:
// timer.id == 0 : 停止Dispather
// timer.id != 0 && timer.endTs == 0: 取消Timer
// timer.id != 0 && timer.endTs != 0: 新建Timer
// timer.id != 0 && timer.endTs != 0 && timer.cb == nil: 更新Timer
func (disp *Dispatcher) doOp(t *dispatcherTimer) bool {
	// 操作A: 停止Dispathcer
	if t.id == 0 {
		return false
	}

	// 操作B: 创建Timer
	if t.endTs != 0 && t.cb != nil {
		disp.place(t)
		return true
	}

	// 操作C: 更新Timer
	if t.endTs != 0 && t.cb == nil {
		// 找到并删除Timer
		oldt := disp.delete(t.id)
		// 重新找合适的框
		if oldt != nil {
			oldt.endTs = t.endTs
			disp.place(oldt)
		} else {
			slog.Error(fmt.Sprintf("delay timer%d, get old timer fail", t.id))
		}
	}

	// 操作D: 删除Timer
	if t.endTs == 0 {
		disp.delete(t.id)
	}
	return true
}

// 删除并返回Timer
func (disp *Dispatcher) delete(timerID int64) *dispatcherTimer {
	for i := timerLevel - 1; i >= 0; i-- {
		slotMap := disp.timerSlots[i]
		if v, ok := slotMap[timerID]; ok {
			delete(slotMap, timerID)
			return v
		}
	}
	return nil
}

// 将Timer放到合适的时间轮中
func (disp *Dispatcher) place(t *dispatcherTimer) {
	diff := t.endTs - utility.ToUTC(utility.Now()).UnixNano()/1e6
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
	for i := timerLevel - 1; i >= 0; i-- {
		if diff >= (timerTick << uint(i)) {
			disp.timerSlots[i][t.id] = t
			break
		}
	}

}

func (disp *Dispatcher) doTick(now time.Time, lastTick int64) int64 {
	// 防止服务器时间手动调整前移后 Timer重复触发
	nowMs := now.UnixNano() / 1e6
	nowTick := utility.ToUTC(utility.Now()).UnixNano() / 1e6 / timerTick
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
		if v.endTs-nowMs < (1 << uint(level) * timerTick) {
			if level != 0 {
				disp.timerSlots[level-1][k] = v
				delete(slotMap, k)
			} else if nowMs > v.endTs {
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
		timerID = utility.NewID().Int64()
	}
	disp.chanOp <- &dispatcherTimer{id: timerID, endTs: timeout, cb: cb}
	return timerID
}

// CancelTimer 取消定时器
func (disp *Dispatcher) CancelTimer(timerID int64) {
	disp.chanOp <- &dispatcherTimer{id: timerID, endTs: 0}
}
