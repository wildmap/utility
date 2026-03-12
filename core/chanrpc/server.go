package chanrpc

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"sync/atomic"

	"github.com/wildmap/utility/xlog"
)

// Server ChanRPC 服务端，接收并处理来自 Client 的 RPC 调用。
//
// 每个模块持有一个 Server 实例，所有外部 RPC 调用通过有缓冲的 ChanCall 通道排队，
// 在模块的事件循环（Skeleton.OnStart）中串行出队处理，从而保证模块内部状态访问无并发竞争。
//
// 架构优势：消息路由通过 functions 哈希表实现 O(1) 查找，
// 相比传统的 switch-case 分发，新增消息类型只需调用 Register 注册一次，扩展成本极低。
type Server struct {
	functions map[uint32]Handler // 消息 ID → 处理函数的路由表，初始化后只读，无需加锁
	ChanCall  chan *CallInfo     // RPC 调用的缓冲通道，容量决定最大可积压的未处理调用数量
	closed    atomic.Bool        // 关闭标志，采用原子操作保证多 goroutine 并发访问时的可见性
}

// NewServer 创建指定缓冲容量的 ChanRPC 服务端。
//
// callLen 决定消息积压的峰值上限：超出后，非阻塞模式的发送方收到 channel full 错误，
// 阻塞模式的发送方等待超时后收到 ErrCallTimeout。
// 应根据模块的消息处理速率和业务峰值流量合理设置此值，过小导致背压，过大增加内存占用。
func NewServer(callLen int) *Server {
	s := new(Server)
	s.functions = map[uint32]Handler{}
	s.ChanCall = make(chan *CallInfo, callLen)
	return s
}

// Register 注册消息处理函数，通过传入 message 实例的类型自动推导消息 ID。
//
// 每种消息类型只允许注册一个处理函数（防止意外覆盖），
// 消息 ID 由 MessageID 函数基于类型全限定名的 BKDR 哈希自动生成，无需手动维护映射表。
// 通常在模块的 OnInit 阶段完成注册，此后路由表只读，访问无需加锁。
func (s *Server) Register(message any, f Handler) error {
	if message == nil {
		return ErrRegisterMsgNil
	}
	if f == nil {
		return ErrRegisterHandlerNil
	}
	messageID := MessageID(message)
	if messageID <= 0 {
		return fmt.Errorf("chanrpc register: invalid message type %v", reflect.TypeOf(message))
	}

	if _, ok := s.functions[messageID]; ok {
		return fmt.Errorf("function ID %v: already registered, type: %v", messageID, reflect.TypeOf(message))
	}
	xlog.Infof("chanrpc register: %v function ID %v", reflect.TypeOf(message), messageID)
	s.functions[messageID] = f
	return nil
}

// exec 执行单次 RPC 调用的核心逻辑：路由到处理函数、执行并回包。
//
// 防御性设计：通过 defer + recover 捕获处理函数内部抛出的 panic，
// 并在 panic 恢复后自动向调用方回包错误，防止业务逻辑异常导致调用方的 Call 永久阻塞。
// hasRet 的 CAS 检查确保 panic 恢复路径与正常执行路径互斥，不会产生重复响应。
func (s *Server) exec(ci *CallInfo) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic: %v", r)
			}
			xlog.Errorf("chanrpc message_id %d exec panic %v\n%s", ci.MessageID(), err, string(debug.Stack()))
		}
		// 确保无论正常返回还是 panic 恢复，调用方都能收到响应，避免死锁
		if ci.hasRet.CompareAndSwap(false, true) {
			_ = ci.ret(&RetInfo{Err: err})
		}
	}()

	// 根据消息 ID 在路由表中 O(1) 查找处理函数
	handler, ok := s.functions[ci.MessageID()]
	if !ok {
		err = fmt.Errorf("chanrpc message_id %d not registered, type: %T", ci.MessageID(), ci.Request)
		return
	}

	ret := handler(ci)
	return ci.ret(ret)
}

// Exec 公开的消息执行入口，在模块的 OnStart 事件循环中逐一调用。
//
// 执行前将 hasRet 重置为 false，允许处理函数通过 CallInfo.ret 延迟响应
// （如异步等待数据库返回后再回包），而不强制在 handler 返回时立即响应。
func (s *Server) Exec(ci *CallInfo) {
	if ci == nil {
		xlog.Warnf("chanrpc exec callInfo is nil")
		return
	}
	ci.hasRet.Store(false)
	if err := s.exec(ci); err != nil {
		xlog.Warnln(err)
	}
}

// IsClosed 检查服务端是否已关闭。
func (s *Server) IsClosed() bool {
	return s.closed.Load()
}

// Close 关闭服务端并清空消息队列，向所有积压的调用方回包 ErrServerClosed 错误。
//
// 使用 CompareAndSwap 保证 Close 的幂等性（重复调用安全，不会 panic）。
// 关闭流程：先将 closed 置为 true 阻断新调用写入 → 再 close(ChanCall) →
// 最后排空队列中的积压消息并逐一回包，防止调用方因无响应而永久等待。
//
// 注意：close(ChanCall) 后立即遍历通道是安全的，
// for range 会在通道排空后自动退出，不会阻塞。
func (s *Server) Close() {
	// CAS 保证只有第一次 Close 调用真正执行关闭逻辑，后续调用直接返回
	if !s.closed.CompareAndSwap(false, true) {
		xlog.Warnf("chanrpc server already closed")
		return
	}

	close(s.ChanCall)

	// 排空队列中尚未处理的调用，向每个调用方返回服务已关闭错误，避免死锁
	for ci := range s.ChanCall {
		_ = ci.ret(&RetInfo{
			Err: ErrServerClosed,
		})
	}
}
