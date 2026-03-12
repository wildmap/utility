// Package timermgr 提供基于多级时间轮算法的高性能定时器调度系统。
//
// 设计思路：通过牺牲少量时间精度（最小粒度 4ms），换取比 time.After/time.NewTimer 更低的
// CPU 开销和 GC 压力。时间轮算法将定时器按剩余时间分层存储，每次 tick 只需扫描
// 当前对应层级的少量定时器，时间复杂度接近 O(1)，完全避免了全量扫描的性能瓶颈。
package timermgr

import (
	"runtime/debug"
	"sync"
	"time"

	"github.com/wildmap/utility/core/idgen"
	"github.com/wildmap/utility/xlog"
	"github.com/wildmap/utility/xtime"
)

// 时间轮配置常量。
const (
	// timerTick 时间轮每次推进的最小时间粒度（毫秒）。
	// 选用 4ms 是因为：① 对游戏逻辑精度足够；② 是 2 的幂次，便于位运算实现高效的层级判断。
	timerTick = 4

	// timerLevel 时间轮总层级数，决定可调度的最大定时时长。
	// 28 级时最大支持 2^28 × 4ms ≈ 12.4 天，完全覆盖游戏服务器的业务需求。
	timerLevel = 28
)

// Dispatcher 多级时间轮定时器分发器，在独立 goroutine 中运行时间轮主循环。
//
// 多级时间轮原理：
//   - 第 i 层（i ∈ [0, timerLevel)）存储剩余时间在 [2^(i-1)×timerTick, 2^i×timerTick) 的定时器
//   - tick 计数的低 i 位全为 0 时扫描第 i 层，实现每层按不同频率触发，避免低层级的频繁全扫描
//
// 并发安全：所有对时间轮数据（timerSlots）的修改均通过 chanOp 串行化到分发器 goroutine，
// 外部调用通过 chanOp 发送操作命令，无需额外加锁。
type Dispatcher struct {
	timerSlots     [timerLevel]map[int64]*dispatcherTimer // 分级时间轮槽位，每层存储对应时间区间的定时器
	chanOp         chan *dispatcherTimer                  // 操作通道：将 Add/Update/Cancel 操作串行化到分发器 goroutine
	ChanTimer      chan *dispatcherTimer                  // 触发通道：定时器到期时投递到此通道，由使用者（TimerMgr）消费
	canceledTimers sync.Map                               // 已取消定时器 ID 的快速过滤集合，防止触发通道中的已投递事件被错误消费
}

// dispatcherTimer 时间轮内部使用的定时器节点，同时复用为操作命令的载体。
type dispatcherTimer struct {
	id    int64       // 定时器唯一 ID；id=0 为内置停止信号
	endTs int64       // 到期绝对时间戳（毫秒）；endTs=0 表示取消操作
	cb    func(int64) // 到期回调函数；cb=nil 表示更新操作（非新建）
}

// Cb 安全执行定时器回调。
//
// 执行后主动将 cb 引用置为 nil，允许 GC 回收回调函数闭包捕获的外部资源，
// 避免已触发的定时器节点因仍被 ChanTimer 引用而造成的内存泄漏。
// panic 由 recover 捕获并记录，保证单个回调异常不会导致分发器崩溃。
func (t dispatcherTimer) Cb() {
	defer func() {
		t.cb = nil // 清除引用，允许 GC 回收回调函数捕获的资源
		if r := recover(); r != nil {
			xlog.Errorf("%v\n%s", r, string(debug.Stack()))
		}
	}()
	t.cb(t.id)
}

// NewDispatcher 创建并初始化多级时间轮分发器。
//
// 参数 l 指定操作通道（chanOp）和触发通道（ChanTimer）的缓冲容量，
// 建议与所在模块的 ChanRPC 缓冲容量保持同一量级，避免背压。
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

// Run 在独立 goroutine 中启动时间轮主循环。
func (disp *Dispatcher) Run() {
	go disp.run()
}

// run 时间轮主循环，每隔 timerTick 毫秒推进一次时间轮并检查到期定时器。
//
// 双通道监听设计：
//   - chanOp：处理增删改定时器操作，由外部通过公开方法投递
//   - tickTimer.C：每 4ms 触发一次，推进时间轮并触发到期定时器
//
// select 的随机选择特性在两个通道同时就绪时提供公平调度，
// 但高频 tick 与大量 chanOp 并发时 tick 可能延迟，这是可接受的精度损耗。
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
				return // doOp 返回 false 表示收到停止信号
			}
		case <-tickTimer.C:
			tickTimer.Reset(timerTick * time.Millisecond)
			lastTick = disp.doTick(xtime.Now(), lastTick)
		}
	}
}

// doOp 处理定时器操作命令，通过约定的字段值区分操作类型。
//
// 操作类型判断规则（优先级从高到低）：
//   - endTs == 0：取消操作（无论 cb 是否为 nil）
//   - id == 0：停止分发器主循环（内置停止信号）
//   - endTs != 0 && cb != nil：新建定时器
//   - endTs != 0 && cb == nil：更新定时器到期时间（加速或延迟）
//
// 复用同一结构体携带不同语义，减少了内存分配，但增加了阅读理解成本，需结合注释理解。
func (disp *Dispatcher) doOp(t *dispatcherTimer) bool {
	// 取消操作：先标记取消（使触发通道中的已投递事件也能被过滤），再从时间轮删除
	if t.endTs == 0 {
		disp.canceledTimers.Store(t.id, struct{}{})
		disp.delete(t.id)
		return true
	}

	// 若定时器已被标记为取消，忽略后续的新建/更新操作（防止 Cancel 后重建的竞态）
	if _, canceled := disp.canceledTimers.Load(t.id); canceled {
		return true
	}

	// id == 0 是约定的内置停止信号，触发后分发器主循环退出
	if t.id == 0 {
		return false
	}

	// 新建定时器：清除可能残留的旧取消标记（防止 Cancel 后立即 NewTimer 的竞态问题），并插入时间轮
	if t.endTs != 0 && t.cb != nil {
		disp.canceledTimers.Delete(t.id)
		disp.place(t)
		return true
	}

	// 更新定时器到期时间：从当前槽位删除旧节点，更新时间戳后重新放入合适的槽位
	if t.endTs != 0 && t.cb == nil {
		oldt := disp.delete(t.id)
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

// delete 从时间轮所有层级中删除指定 ID 的定时器节点，并清理取消标记。
//
// 从高层级向低层级逐层扫描：虽然定时器理论上只在一层，但为保证健壮性做全量扫描。
// 找到并删除后清理 canceledTimers 中对应的取消标记，防止 sync.Map 无限积累。
// 未找到定时器时同样清理标记，因为定时器可能已触发并从时间轮移除。
func (disp *Dispatcher) delete(timerID int64) *dispatcherTimer {
	for i := timerLevel - 1; i >= 0; i-- {
		if v, ok := disp.timerSlots[i][timerID]; ok {
			delete(disp.timerSlots[i], timerID)
			disp.canceledTimers.Delete(timerID) // 物理删除成功后同步清理取消标记
			return v
		}
	}
	disp.canceledTimers.Delete(timerID) // 未找到时也清理，防止 sync.Map 内存无限积累
	return nil
}

// place 将定时器放入时间轮的合适层级。
//
// 层级选择算法（贪心策略）：计算剩余时间 diff，找到满足 diff ≤ 2^i × timerTick 的最小层级 i，
// 确保定时器被放在"刚好能容纳其剩余时间"的最低层级，提高调度精度。
//
// 已到期处理：若 diff ≤ 0，直接以非阻塞方式投递到 ChanTimer，
// 防止分发器因 ChanTimer 满而阻塞（丢弃时允许：下次 doTick 会重新检查到期定时器）。
func (disp *Dispatcher) place(t *dispatcherTimer) {
	if _, canceled := disp.canceledTimers.Load(t.id); canceled {
		return
	}

	diff := t.endTs - xtime.Now().UnixMilli()
	if diff <= 0 {
		// 已到期，直接触发，非阻塞避免 ChanTimer 满时阻塞分发器主循环
		select {
		case disp.ChanTimer <- t:
		default:
		}
		return
	}
	if diff < timerTick {
		diff = timerTick // 保底最小粒度，防止定时器在 level=0 中被无限反复检测
	}
	// 从低层级向高层级查找合适的槽位，timerTick << i 等价于 timerTick × 2^i
	for i := range timerLevel {
		if diff <= (timerTick << uint(i)) {
			disp.timerSlots[i][t.id] = t
			break
		}
	}

}

// doTick 推进时间轮，触发所有已到期的定时器。
//
// 防时钟回拨：若 nowTick ≤ lastTick，直接返回，不做任何操作，避免重复触发。
// 防时钟跳变（forward）：若系统时钟突然向前跳变，nowTick - lastTick 可能很大，
// 此时采用逐步推进策略（每次 lastTick++），确保中间层级的定时器降级逻辑不被跳过。
// 逐步推进保证了"定时器从高层降到低层 → 再触发"的完整流程，防止遗漏定时器。
func (disp *Dispatcher) doTick(now time.Time, lastTick int64) int64 {
	nowMs := now.UnixMilli()
	nowTick := nowMs / timerTick
	if nowTick-lastTick < 1 {
		return nowTick
	}

	for {
		lastTick++
		// 层级扫描规则：第 i 层在 lastTick 的低 i 位全为 0 时触发
		// 等价于每隔 2^i 个 tick 扫描一次第 i 层（高层扫描频率更低，与其存储的长剩余时间匹配）
		for i := timerLevel - 1; i >= 0; i-- {
			mask := (1 << uint(i)) - 1
			if lastTick&int64(mask) == 0 {
				disp.trigger(nowMs, i)
			}
		}

		if lastTick >= nowTick {
			break
		}
	}
	return nowTick
}

// trigger 扫描指定层级的定时器，将已满足降级条件的定时器下移至更低层级，
// 或将已到期的最低层（level=0）定时器投递到触发通道。
//
// 降级逻辑：若定时器的剩余时间已缩短至低一层级的范围内（< 2^level × timerTick），
// 将其移至 level-1 层，使其在更短的周期内被精确检测并触发。
// 最低层（level=0）到期后，以非阻塞方式投递，发送失败则等待下次 tick 重试，
// 保证分发器 goroutine 不会被满通道阻塞。
func (disp *Dispatcher) trigger(nowMs int64, level int) {
	slotMap := disp.timerSlots[level]
	for k, v := range slotMap {
		// 快速过滤已取消的定时器，避免触发失效回调
		if _, canceled := disp.canceledTimers.Load(k); canceled {
			delete(slotMap, k)
			disp.canceledTimers.Delete(k) // 物理删除后同步清理取消标记
			continue
		}

		// timerTick << uint(level) = timerTick × 2^level，使用位移避免整数溢出
		if v.endTs-nowMs < ((1 << uint(level)) * timerTick) {
			if level != 0 {
				// 将定时器下移至更精确的层级，使其在更短的扫描周期内被触发
				disp.timerSlots[level-1][k] = v
				delete(slotMap, k)
			} else if nowMs >= v.endTs {
				// 最低层且已到期：非阻塞投递，ChanTimer 满时本次跳过，下次 tick 重试
				select {
				case disp.ChanTimer <- v:
					delete(slotMap, k)
				default:
				}
			}
		}
	}
}

// Stop 向分发器投递内置停止信号（id=0, endTs=0），使主循环退出。
func (disp *Dispatcher) Stop() {
	disp.chanOp <- &dispatcherTimer{id: 0, endTs: 0}
}

// UpdateTimer 向分发器投递定时器到期时间更新命令（cb=nil 表示更新操作）。
func (disp *Dispatcher) UpdateTimer(timerID, newEndTs int64) {
	disp.chanOp <- &dispatcherTimer{id: timerID, endTs: newEndTs}
}

// NewTimer 向分发器投递新建定时器命令。
//
// timerID 为 0 时自动调用 utility.NextID() 生成全局唯一 ID，
// 保证多模块并发创建定时器时 ID 不冲突。
func (disp *Dispatcher) NewTimer(timerID, timeout int64, cb func(int64)) int64 {
	if timerID == 0 {
		timerID = idgen.NextID().Int64()
	}
	disp.chanOp <- &dispatcherTimer{id: timerID, endTs: timeout, cb: cb}
	return timerID
}

// CancelTimer 取消定时器，采用双重取消机制保证可靠性。
//
// 双重机制：
//  1. 立即在 canceledTimers 中标记取消，使 ChanTimer 中已投递但尚未消费的到期事件被过滤
//  2. 异步通过 chanOp 发送删除命令，从时间轮数据结构中物理删除定时器节点
//
// 之所以需要步骤 1：时间轮触发事件是先投递到 ChanTimer 再删除节点的，
// 若仅依赖步骤 2，已投递到 ChanTimer 的事件仍可能被消费，导致取消后回调仍被执行。
func (disp *Dispatcher) CancelTimer(timerID int64) {
	disp.canceledTimers.Store(timerID, struct{}{}) // 立即标记，触发通道中的已投递事件也会被过滤
	disp.chanOp <- &dispatcherTimer{id: timerID, endTs: 0}
}
