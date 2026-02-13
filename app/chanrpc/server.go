package chanrpc

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"sync/atomic"

	"github.com/wildmap/utility/xlog"
)

// Server 代理服务器
type Server struct {
	functions map[uint32]Handler
	ChanCall  chan *CallInfo
	closed    atomic.Bool // 关闭标志
}

// NewServer 新建服务器
func NewServer(callLen int) *Server {
	s := new(Server)
	s.functions = map[uint32]Handler{}
	s.ChanCall = make(chan *CallInfo, callLen)
	return s
}

// Register 向服务器注册处理函数 根据id索引
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

// exec 实际执行
func (s *Server) exec(ci *CallInfo) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic: %v", r)
			}
			xlog.Errorf("chanrpc message_id %d exec panic %v\n%s", ci.MessageID(), err, string(debug.Stack()))

			// 如果还没有返回，则返回错误
			if ci.hasRet.CompareAndSwap(false, true) {
				_ = ci.ret(&RetInfo{Err: err})
			}
		}
	}()

	// 根据id取handler
	handler, ok := s.functions[ci.MessageID()]
	if !ok {
		return fmt.Errorf("chanrpc message_id %d not registered, type: %T", ci.MessageID(), ci.Request)
	}

	handler(ci)
	return nil
}

// Exec 执行
func (s *Server) Exec(ci *CallInfo) {
	if ci == nil {
		xlog.Warnf("chanrpc exec callInfo is nil")
		return
	}
	ci.hasRet.Store(false)
	if err := s.exec(ci); err != nil {
		xlog.Warnln(err)
		ci.RetWithError(nil, err)
	}
}

// IsClosed 检查服务器是否已关闭
func (s *Server) IsClosed() bool {
	return s.closed.Load()
}

// Close 关闭服务器
func (s *Server) Close() {
	// 先设置关闭标志，防止新的调用进入
	if !s.closed.CompareAndSwap(false, true) {
		xlog.Warnf("chanrpc server already closed")
		return
	}

	// 关闭通道
	close(s.ChanCall)

	// 排空通道中的剩余消息
	for ci := range s.ChanCall {
		_ = ci.ret(&RetInfo{
			Err: ErrServerClosed,
		})
	}
}
