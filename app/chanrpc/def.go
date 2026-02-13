package chanrpc

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/wildmap/utility/xlog"
)

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

// Handler 方法句柄 处理CallInfo
type Handler func(ci *CallInfo)

// Callback 回调
type Callback func(ri *RetInfo)

// CallInfo 调用参数
type CallInfo struct {
	messageID uint32        // 消息类型id
	Request   any           // 入参
	chanRet   chan *RetInfo // 结果信息返回通道
	callback  Callback      // 回调
	hasRet    atomic.Bool   // 是否已经返回 由被调用方使用
}

// Ret 调用请求的回调
func (ci *CallInfo) Ret(ret any) {
	// 检查回调是否已经使用过
	if !ci.hasRet.CompareAndSwap(false, true) {
		xlog.Warnf("chanrpc message_id %d can not ret twice, %s", ci.MessageID(), string(debug.Stack()))
		return
	}

	// 封装参数 执行回调
	if err := ci.ret(&RetInfo{Ack: ret}); err != nil {
		xlog.Warnf("chanrpc message_id %d ret error %v", ci.MessageID(), err)
	}
}

// RetWithError 带错误的返回
func (ci *CallInfo) RetWithError(ret any, e error) {
	// 检查回调是否已经使用过
	if !ci.hasRet.CompareAndSwap(false, true) {
		xlog.Warnf("chanrpc message_id %d can not ret twice %s", ci.MessageID(), string(debug.Stack()))
		return
	}
	// 封装参数 执行回调
	if err := ci.ret(&RetInfo{Ack: ret, Err: e}); err != nil {
		xlog.Warnf("chanrpc message_id %d ret error %v", ci.MessageID(), err)
	}
}

// ret 调用请求的回调
func (ci *CallInfo) ret(ri *RetInfo) (err error) {
	// 检查返回通道
	if ci.chanRet == nil {
		return nil
	}
	// 错误捕获
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
			xlog.Errorf("chanrpc message_id %d ret err %v\n%s", ci.MessageID(), err, string(debug.Stack()))
		}
	}()
	// 封装参数 将结果信息放入返回通道
	ri.callback = ci.callback

	// 使用select防止阻塞
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case ci.chanRet <- ri:
		return nil
	case <-timer.C:
		return fmt.Errorf("%w message_id %d", ErrRetTimeout, ci.MessageID())
	}
}

// MessageID 调用消息ID
func (ci *CallInfo) MessageID() uint32 {
	return ci.messageID
}

// RetInfo 结果信息
type RetInfo struct {
	Ack      any      `json:"Ack"` // 结果值 作为回调函数的入参
	Err      error    `json:"Err"` // 错误
	callback Callback // 回调
}

// MessageID 返回消息的结果(回调入参)类型ID
func (ri *RetInfo) MessageID() uint32 {
	if ri.Err != nil || ri.Ack == nil {
		return 0
	}
	return MessageID(ri.Ack)
}
