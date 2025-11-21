package chanrpc

import (
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wildmap/utility"
	"github.com/wildmap/utility/xlog"
)

// Handler 方法句柄 处理CallInfo
type Handler func(ci *CallInfo)

// Callback 回调
type Callback func(ri *RetInfo)

// Server 代理服务器
type Server struct {
	functions map[uint32]Handler
	ChanCall  chan *CallInfo
}

// CallInfo 调用参数
type CallInfo struct {
	id        uint32        // 消息类型id
	Req       any           // 入参
	chanRet   chan *RetInfo // 结果信息返回通道
	cb        Callback      // 回调
	hasRet    atomic.Bool   // 是否已经返回 由被调用方使用
	sessionID int64
}

// Ret 调用请求的回调
func (ci *CallInfo) Ret(ret any) {
	// 检查回调是否已经使用过
	if !ci.hasRet.CompareAndSwap(false, true) {
		xlog.Errorf("chanrpc msgid %d can not ret twice, %s", ci.id, string(debug.Stack()))
		return
	}

	// 封装参数 执行回调
	if err := ci.ret(&RetInfo{Ack: ret}); err != nil {
		xlog.Errorf("chanrpc msgid %d ret error %v", ci.id, err)
	}
}

// RetWithError 带错误的返回
func (ci *CallInfo) RetWithError(ret any, e error) {
	// 检查回调是否已经使用过
	if !ci.hasRet.CompareAndSwap(false, true) {
		xlog.Errorf("chanrpc msgid %d can not ret twice %s", ci.id, string(debug.Stack()))
		return
	}
	// 封装参数 执行回调
	if err := ci.ret(&RetInfo{Ack: ret, Err: e}); err != nil {
		xlog.Errorf("chanrpc msgid %d ret error %v", ci.id, err)
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
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic: %v", r)
			}
			xlog.Errorf("chanrpc msgid %d ret err %v\n%s", ci.id, err, string(debug.Stack()))
		}
	}()
	// 封装参数 将结果信息放入返回通道
	ri.cb = ci.cb
	ri.clientSessionID = ci.sessionID

	// 使用select防止阻塞
	select {
	case ci.chanRet <- ri:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("chanrpc ret timeout")
	}
}

// GetMsgID 调用消息ID
func (ci *CallInfo) GetMsgID() uint32 {
	return ci.id
}

// RetInfo 结果信息
type RetInfo struct {
	Ack             any      `json:"Ack"` // 结果值 作为回调函数的入参
	Err             error    `json:"Err"` // 错误
	cb              Callback // 回调
	clientSessionID int64
}

// GetMsgID 返回消息的结果(回调入参)类型ID
func (ri *RetInfo) GetMsgID() uint32 {
	if ri.Err != nil || ri.Ack == nil {
		return 0
	}
	return utility.MsgID(ri.Ack)
}

// Client 客户端
type Client struct {
	chanCall        chan *CallInfo   // 调用信息通道
	chanSyncRet     chan *RetInfo    // 同步调用结果通道
	ChanASynRet     chan *RetInfo    // 异步调用结果通道
	pendingASynCall int32            // 最大待处理异步调用（使用原子操作）
	callList        map[int64]string // 待处理异步调用
	sessionID       int64            // 会话ID（使用原子操作）
	closed          int32            // 关闭标志（原子操作）
}

// NewServer 新建服务器
func NewServer(callLen int) *Server {
	s := new(Server)
	s.functions = map[uint32]Handler{}
	s.ChanCall = make(chan *CallInfo, callLen)
	return s
}

// Register 向服务器注册处理函数 根据id索引
func (s *Server) Register(msg any, f Handler) error {
	if msg == nil || f == nil {
		return fmt.Errorf("chanrpc register: msg and handler cannot be nil")
	}
	msgID := utility.MsgID(msg)
	if msgID <= 0 {
		return fmt.Errorf("chanrpc register: invalid msg type %v", reflect.TypeOf(msg))
	}
	if _, ok := s.functions[msgID]; ok {
		return fmt.Errorf("function ID %v: already registered, type: %v", msgID, reflect.TypeOf(msg))
	}
	s.functions[msgID] = f
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
			xlog.Errorf("chanrpc msgid %d exec panic %v\n%s", ci.id, err, string(debug.Stack()))

			// 如果还没有返回，则返回错误
			if ci.hasRet.CompareAndSwap(false, true) {
				_ = ci.ret(&RetInfo{Err: err})
			}
		}
	}()

	// 根据id取handler
	handler, ok := s.functions[ci.id]
	if !ok {
		return fmt.Errorf("msg id %d not registered, type: %T", ci.id, ci.Req)
	}

	handler(ci)
	return nil
}

// Exec 执行
func (s *Server) Exec(ci *CallInfo) {
	if ci == nil {
		xlog.Errorf("chanrpc exec: CallInfo is nil")
		return
	}
	ci.hasRet.Store(false)
	if err := s.exec(ci); err != nil {
		xlog.Errorf("chanrpc msgid %d exec error %v", ci.id, err)
	}
}

// Cast 直接投递消息 忽略任何错误和返回值
func (s *Server) Cast(req any) error {
	id := utility.MsgID(req)
	if id <= 0 {
		return errors.New("invalid message type")
	}
	return call(s.ChanCall, &CallInfo{
		id:  id,
		Req: req,
	}, false)
}

// Call 启动一个client来进行调用
func (s *Server) Call(req any) *RetInfo {
	return s.Open(0).Call(req)
}

// Close 关闭服务器
func (s *Server) Close() {
	close(s.ChanCall)
	for ci := range s.ChanCall {
		_ = ci.ret(&RetInfo{
			Err: errors.New("chanrpc server closed"),
		})
	}
}

// Open 启动一个客户端
func (s *Server) Open(callLen int) *Client {
	c := NewClient(callLen)
	_ = c.Attach(s)
	return c
}

// NewClient 新建客户端 设置异步结果通道的最大缓冲值
func NewClient(callLen int) *Client {
	c := &Client{
		chanSyncRet: make(chan *RetInfo, 1),
		ChanASynRet: make(chan *RetInfo, callLen),
		callList:    make(map[int64]string),
	}
	return c
}

// Attach 将client的请求通道依附于服务器
func (c *Client) Attach(s *Server) error {
	if s == nil {
		return errors.New("chanrpc attach: server cannot be nil")
	}
	c.chanCall = s.ChanCall
	return nil
}

// IsClosed 检查客户端是否已关闭
func (c *Client) IsClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

// Call 同步有结果调用
func (c *Client) Call(req any) *RetInfo {
	if c.IsClosed() {
		return &RetInfo{Err: errors.New("chanrpc client closed")}
	}
	id := utility.MsgID(req)
	if id <= 0 {
		return &RetInfo{Err: errors.New("invalid message type")}
	}

	err := call(c.chanCall, &CallInfo{
		id:      id,
		Req:     req,
		chanRet: c.chanSyncRet,
	}, true)
	if err != nil {
		return &RetInfo{Err: err}
	}

	ri := <-c.chanSyncRet
	return ri
}

// ASynCall 异步调用
func (c *Client) ASynCall(req any, cb Callback) error {
	if c.IsClosed() {
		return errors.New("chanrpc client closed")
	}

	id := utility.MsgID(req)
	if id <= 0 {
		return errors.New("invalid message type")
	}

	// 原子递增sessionID
	sessionID := atomic.AddInt64(&c.sessionID, 1)

	err := call(c.chanCall, &CallInfo{
		id:        id,
		Req:       req,
		chanRet:   c.ChanASynRet,
		cb:        cb,
		sessionID: sessionID,
	}, false)
	if err != nil {
		return err
	}

	name := reflect.TypeOf(req).String()
	c.callList[sessionID] = name

	// 原子递增待处理计数
	atomic.AddInt32(&c.pendingASynCall, 1)
	return nil
}

// execCb 执行回调
func execCb(ri *RetInfo) {
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("chanrpc  callback panic: %v\n%s", r, string(debug.Stack()))
		}
	}()

	if ri.cb != nil {
		ri.cb(ri)
	}
}

// Cb 执行回调
func (c *Client) Cb(ri *RetInfo) {
	// 原子递减待处理计数
	atomic.AddInt32(&c.pendingASynCall, -1)
	delete(c.callList, ri.clientSessionID)
	execCb(ri)
}

// Close 关闭client
func (c *Client) Close() {
	// 设置关闭标志
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		xlog.Warnf("chanrpc client already closed")
		return
	}

	pending := atomic.LoadInt32(&c.pendingASynCall)
	xlog.Infof("closing chanrpc client pending_calls %d", pending)

	if pending == 0 {
		return
	}

	// 使用WaitGroup等待所有异步调用完成
	var wg sync.WaitGroup
	wg.Go(func() {
		timeout := time.After(5 * time.Second)

		for {
			// 检查是否还有待处理的调用
			if atomic.LoadInt32(&c.pendingASynCall) <= 0 {
				return
			}

			select {
			case ret := <-c.ChanASynRet:
				c.Cb(ret)
			case <-timeout:
				// 超时后强制清理
				remaining := atomic.LoadInt32(&c.pendingASynCall)
				xlog.Warnf("chanrpc client close timeout remaining_calls %d", remaining)

				// 打印剩余的调用信息
				for sid, _call := range c.callList {
					xlog.Warnf("pending call not finished session_id %d call %s", sid, _call)
				}

				// 强制清零
				atomic.StoreInt32(&c.pendingASynCall, 0)
				return
			}
		}
	})

	// 等待goroutine完成
	wg.Wait()
	xlog.Infof("chanrpc client closed successfully")
}

// Idle 判断是否空闲
func (c *Client) Idle() bool {
	return atomic.LoadInt32(&c.pendingASynCall) == 0
}

// PendingCount 获取待处理的异步调用数量
func (c *Client) PendingCount() int32 {
	return atomic.LoadInt32(&c.pendingASynCall)
}

// call 调用,分阻塞与非阻塞模式,仅仅将请求放入请求通道
func call(chanCall chan *CallInfo, ci *CallInfo, block bool) (err error) {
	if chanCall == nil {
		return errors.New("chanrpc call: channel is nil")
	}
	if ci == nil {
		return errors.New("chanrpc call: CallInfo is nil")
	}

	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic: %v\n%s", r, string(debug.Stack()))
			}
		}
	}()

	if block {
		// 阻塞模式，添加超时控制
		select {
		case chanCall <- ci:
			return nil
		case <-time.After(5 * time.Second):
			return errors.New("chanrpc call blocked timeout")
		}
	}

	// 非阻塞模式
	select {
	case chanCall <- ci:
		return nil
	default:
		reqType := "unknown"
		if ci.Req != nil {
			reqType = reflect.TypeOf(ci.Req).String()
		}
		return fmt.Errorf("server chanrpc channel full, msg %v %+v", reqType, ci.Req)
	}
}
