package chanrpc

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/wildmap/utility/xlog"
)

// ChanRPC 相关预定义错误，覆盖客户端/服务端关闭、参数非法、超时等各种异常场景。
// 使用具名变量而非 fmt.Errorf 字面量，便于调用方通过 errors.Is 精确判断错误类型。
var (
	ErrServerClosed       = errors.New("chanrpc: server closed")
	ErrClientClosed       = errors.New("chanrpc: client closed")
	ErrServerNil          = errors.New("chanrpc: server cannot be nil")
	ErrCallbackNil        = errors.New("chanrpc: callback cannot be nil")
	ErrInvalidMsgType     = errors.New("chanrpc: invalid message type")
	ErrCallTimeout        = errors.New("chanrpc: call timeout")
	ErrRetTimeout         = errors.New("chanrpc: ret timeout")
	ErrRegisterMsgNil     = errors.New("chanrpc: register message cannot be nil")
	ErrRegisterHandlerNil = errors.New("chanrpc: register handler cannot be nil")
	ErrCallChannelNil     = errors.New("chanrpc: call channel is nil")
	ErrCallInfoNil        = errors.New("chanrpc: call CallInfo is nil")
)

// Handler RPC 消息处理函数类型，接收调用信息并返回结果信息。
//
// 处理函数在 Server 所在模块的 goroutine 中串行执行，天然保证对模块内部状态的无锁访问。
// 若处理逻辑无需向调用方回包（如 Cast），可返回 nil。
type Handler func(ci *CallInfo) (ri *RetInfo)

// Callback 异步调用的回调函数类型，在 Client 所在模块的 goroutine 中执行。
//
// 回调与业务逻辑运行在同一 goroutine，无需为访问模块状态加锁，简化了并发编程模型。
type Callback func(ri *RetInfo)

// CallInfo 封装一次 RPC 调用的完整上下文信息。
//
// chanRet 和 callback 配合使用：同步调用时 chanRet 为独立单元素 channel，callback 为 nil；
// 异步调用时 chanRet 为 Client.ChanAsyncRet，callback 为调用方注册的回调；
// Cast 时两者均为 nil，Server 处理后不做任何响应。
//
// hasRet 通过 atomic.Bool 的 CAS 语义实现防重复响应：
// 正常路径和 panic 恢复路径都会尝试响应，CAS 保证只有第一次成功。
type CallInfo struct {
	Request   any           `json:"request"` // 请求数据，业务 handler 的输入
	messageID uint32        // 消息类型 ID，用于路由到对应的 Handler
	chanRet   chan *RetInfo // 响应通道：同步调用时为独立 channel，异步调用时为 Client.ChanAsyncRet
	callback  Callback      // 异步调用的回调函数，同步调用时为 nil
	hasRet    atomic.Bool   // 防重复响应标志，通过 CAS 操作保证并发安全
}

// ret 向调用方发送响应结果，通过 hasRet CAS 防止同一次调用被重复响应。
//
// 采用 5 秒超时的有缓冲发送策略：
//   - 正常情况：调用方的 chanRet 容量为 1，发送立即成功
//   - 异常情况：调用方已超时离开，超时后记录错误并返回，避免 goroutine 永久挂起
//
// 若 chanRet 为 nil（Cast 调用），直接返回 nil，不做任何操作。
func (ci *CallInfo) ret(ri *RetInfo) (err error) {
	if ci.chanRet == nil {
		return nil
	}

	// CompareAndSwap(false → true) 保证只有第一次 ret 调用成功，后续调用均被忽略
	if !ci.hasRet.CompareAndSwap(false, true) {
		xlog.Warnf("chanrpc message_id %d can not ret twice, %s", ci.MessageID(), string(debug.Stack()))
		return
	}

	// 捕获向已关闭 channel 发送时可能触发的 panic（Server.Close 后仍有调用在处理中）
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
			xlog.Errorf("chanrpc message_id %d ret err %v\n%s", ci.MessageID(), err, string(debug.Stack()))
		}
	}()

	if ri == nil {
		ri = new(RetInfo)
	}

	// 将回调函数附加到响应对象，由 Client.AsyncCallback 在调用方 goroutine 中执行
	ri.callback = ci.callback

	// 带超时的非阻塞发送，防止调用方已超时离开导致 goroutine 永久阻塞
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case ci.chanRet <- ri:
		return nil
	case <-timer.C:
		return fmt.Errorf("%w message_id %d", ErrRetTimeout, ci.MessageID())
	}
}

// MessageID 返回本次调用的消息类型 ID。
func (ci *CallInfo) MessageID() uint32 {
	return ci.messageID
}

// RetInfo 封装 RPC 调用的响应数据，同时作为异步回调的上下文载体。
type RetInfo struct {
	Ack      any      `json:"Ack"` // 响应业务数据，作为 Callback 的输入参数
	Err      error    `json:"Err"` // 调用或处理过程中发生的错误
	callback Callback // 异步回调函数引用，由 Client.AsyncCallback 触发执行
}

// MessageID 返回响应数据（Ack）的类型 ID，用于异步回调场景下的消息路由。
//
// 当 Ack 为 nil 或调用本身存在错误时，返回 0 表示无效消息 ID，
// 调用方可据此区分正常响应与错误响应，避免误路由。
func (ri *RetInfo) MessageID() uint32 {
	if ri.Err != nil || ri.Ack == nil {
		return 0
	}
	return MessageID(ri.Ack)
}
