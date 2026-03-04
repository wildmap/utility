package core

import (
	"context"

	"github.com/wildmap/utility/core/chanrpc"
	"github.com/wildmap/utility/core/timermgr"
	"github.com/wildmap/utility/xlog"
)

// IRPC 定义跨模块 RPC 调用的接口，提供三种调用语义覆盖不同并发场景。
//
// 调用模式对比：
//   - Cast：单向投递，无响应，吞吐最高，适合通知/事件
//   - AsyncCall：异步调用，回调在调用方 goroutine 执行，无锁安全，推荐使用
//   - Call：同步阻塞，有死锁风险，仅在确认无循环依赖时使用
type IRPC interface {
	// Cast 单向消息投递，不等待结果，适合日志上报、事件通知等不需要响应的场景。
	Cast(mod string, req any)
	// Call 同步 RPC 调用，阻塞等待对端处理完成并返回结果。
	// 警告：若调用链形成环（A→B→A），将导致死锁，生产环境应优先使用 AsyncCall。
	Call(mod string, req any) *chanrpc.RetInfo
	// AsyncCall 异步 RPC 调用，立即返回，结果通过 cb 回调在调用方 goroutine 处理。
	// 回调在事件循环中串行执行，可安全访问模块内部状态，无需加锁。
	AsyncCall(mod string, req any, cb chanrpc.Callback) error
}

// ITimer 定义定时器管理接口，支持一次性定时器和周期性 Ticker 的完整生命周期管理。
type ITimer interface {
	// RegisterTimer 注册指定类型定时器的处理函数，同 kind 仅能注册一个处理器（后注册覆盖前者）。
	RegisterTimer(kind string, handler timermgr.TimerHandler)
	// NewTimer 创建一次性定时器，duraMs 毫秒后触发一次，返回定时器 ID。
	NewTimer(duraMs int64, kind string, metadata map[string]string) int64
	// NewTicker 创建周期性定时器，每隔 duraMs 毫秒触发一次，返回定时器 ID。
	NewTicker(duraMs int64, kind string, metadata map[string]string) int64
	// AccTimer 加速指定定时器，提前其触发时间。
	AccTimer(id int64, kind timermgr.AccKind, value int64) error
	// DelayTimer 延迟指定定时器，推迟其触发时间。
	DelayTimer(id int64, kind timermgr.AccKind, value int64) (err error)
	// CancelTimer 取消指定 ID 的定时器，对已触发或已取消的定时器调用是安全的（幂等）。
	CancelTimer(id int64)
}

// Skeleton 模块骨架，将 ChanRPC（服务端/客户端）和定时器管理器整合为统一的事件驱动框架。
//
// 核心设计思想（Actor 模型）：
// 所有事件（RPC 调用、异步回调、定时器）在单一 goroutine（OnStart）中串行处理，
// 彻底消除模块内部的并发竞争，开发者无需为访问模块状态加任何锁，极大降低了复杂度。
//
// 使用方式：业务模块内嵌 Skeleton，重写 OnInit 注册处理函数，重写 OnDestroy 清理资源，
// 无需重写 OnStart 和 ChanRPC（Skeleton 已提供默认实现）。
type Skeleton struct {
	name   string
	timer  *timermgr.TimerMgr // 定时器管理器，负责创建、调度和取消定时任务
	server *chanrpc.Server    // ChanRPC 服务端，接收并路由来自其他模块的 RPC 调用
	client *chanrpc.Client    // ChanRPC 客户端，向其他模块发起 RPC 调用
}

// NewSkeleton 创建模块骨架，初始化 ChanRPC 和定时器组件。
//
// 各组件缓冲区均为 10000，适合高并发游戏服务器场景下的消息吞吐需求。
// 若某模块的消息量远超此值，需根据业务峰值流量调整，过小会导致背压和调用方超时。
func NewSkeleton(name string) *Skeleton {
	s := &Skeleton{
		name:   name,
		server: chanrpc.NewServer(10000),
		client: chanrpc.NewClient(10000),
		timer:  timermgr.NewTimerMgr(10000),
	}
	return s
}

// Name 返回模块名称，实现 IModule.Name 接口。
func (s *Skeleton) Name() string {
	return s.name
}

// OnRun 启动模块事件循环，阻塞至 ctx 被取消（即框架调用 cancel）。
//
// 事件循环采用 select 多路复用以下三类事件，保证在单一 goroutine 内串行处理：
//  1. ctx.Done()：接收框架的停止信号，触发模块关闭流程
//  2. ChanAsyncRet：处理本模块发起的异步 RPC 调用的返回结果（执行注册的 Callback）
//  3. ChanCall：处理其他模块发来的 RPC 调用请求（查找并执行已注册的 Handler）
//  4. ChanTimer：处理到期的定时器事件（执行注册的 TimerHandler，并自动续期 Ticker）
//
// 单 goroutine 串行处理是性能与正确性权衡的结果：
// 牺牲了 CPU 并行利用率，换取了零锁开销和极低的编程复杂度。
func (s *Skeleton) OnRun(ctx context.Context) {
	s.timer.Run()
	for {
		select {
		case <-ctx.Done():
			s.close()
			xlog.Infof("%s stopped", s.name)
			return
		case ri := <-s.client.ChanAsyncRet:
			s.client.AsyncCallback(ri)
		case ci := <-s.server.ChanCall:
			s.server.Exec(ci)
		case t := <-s.timer.ChanTimer():
			t.Cb()
		}
	}
}

// close 在模块退出前有序清理资源：停止定时器 → 关闭 RPC 服务端 → 等待异步调用完成。
//
// 轮询等待异步回调（!Idle）：直到所有发出的异步调用都收到响应并执行完回调，
// 防止未处理的回调在模块销毁后被执行时访问已释放的资源。
// 每次调用 client.Close 会处理当前 ChanAsyncRet 中的回调，Idle 检查保证全部处理完毕才退出。
func (s *Skeleton) close() {
	s.timer.Stop()
	s.server.Close()
	// 循环等待，直到客户端所有异步回调都处理完毕（Idle），防止未处理的回调泄漏
	for !s.client.Idle() {
		s.client.Close()
		xlog.Infof("%s skeleton client close ", s.Name())
	}
}

// RegisterTimer 注册指定 kind 类型的定时器处理函数，通常在 OnInit 中调用完成所有注册。
func (s *Skeleton) RegisterTimer(kind string, handler timermgr.TimerHandler) {
	s.timer.RegisterTimer(kind, handler)
}

// NewTimer 创建一次性定时器，duraMs 毫秒后触发一次，自动生成 ID。
func (s *Skeleton) NewTimer(duraMs int64, kind string, metadata map[string]string) int64 {
	return s.timer.NewTimer(duraMs, kind, metadata)
}

// NewTicker 创建周期性定时器，每隔 duraMs 毫秒触发一次，触发后自动续期直到被取消。
//
// id 为 0 时自动生成新 ID；传入已有 ID 时复用该定时器（覆盖更新周期），
// 可用于动态调整已有 Ticker 的触发频率，无需先取消再重建。
func (s *Skeleton) NewTicker(id int64, duraMs int64, kind string, metadata map[string]string) int64 {
	return s.timer.NewTicker(id, duraMs, kind, metadata)
}

// AccTimer 按指定方式加速定时器，提前其触发时间。
func (s *Skeleton) AccTimer(id int64, kind timermgr.AccKind, value int64) error {
	return s.timer.AccTimer(id, kind, value)
}

// DelayTimer 按指定方式延迟定时器，推迟其触发时间。
func (s *Skeleton) DelayTimer(id int64, kind timermgr.AccKind, value int64) (err error) {
	return s.timer.DelayTimer(id, kind, value)
}

// CancelTimer 取消指定 ID 的定时器，同时清理业务层元数据，对已触发/已取消的定时器调用安全（幂等）。
func (s *Skeleton) CancelTimer(id int64) {
	s.timer.CancelTimer(id)
}

// ChanRPC 返回模块的 ChanRPC 服务端，供框架注册到模块映射表，以及外部模块通过 GetChanRPC 获取后投递消息。
func (s *Skeleton) ChanRPC() *chanrpc.Server {
	return s.server
}

// RegisterChanRPC 注册 RPC 消息处理函数，通过 msg 的类型自动推导消息 ID 并完成路由注册。
//
// 通常在 OnInit 中批量注册，注册完成后路由表不再变更，访问无需加锁。
func (s *Skeleton) RegisterChanRPC(msg any, f chanrpc.Handler) error {
	return s.server.Register(msg, f)
}

// AsyncCall 向指定模块发起异步 RPC 调用，结果通过 cb 回调在本模块事件循环中执行。
//
// 回调在 OnStart 的 select 循环中消费 ChanAsyncRet 时执行，
// 与模块其他事件处理串行，无并发问题，可安全访问模块内部状态。
func (s *Skeleton) AsyncCall(mod string, req any, cb chanrpc.Callback) error {
	server := defaultApp.GetChanRPC(mod)
	return s.client.AsyncCall(server, req, cb)
}

// Cast 向指定模块投递单向消息，不等待响应，适合日志记录、事件通知等无需确认的场景。
func (s *Skeleton) Cast(mod string, req any) {
	server := defaultApp.GetChanRPC(mod)
	s.client.Cast(server, req)
}

// Call 向指定模块发起同步 RPC 调用，阻塞当前模块的事件处理直到收到响应。
//
// 危险提示：Call 会阻塞本模块对其他消息的处理；
// 若 A 调用 B，同时 B 也在等待 A 的响应，则形成死锁，需通过仔细的调用关系分析来规避。
// 在事件循环中应优先使用 AsyncCall，仅在调用关系明确单向且不存在环路时才使用 Call。
func (s *Skeleton) Call(mod string, req any) *chanrpc.RetInfo {
	server := defaultApp.GetChanRPC(mod)
	return s.client.Call(server, req)
}
