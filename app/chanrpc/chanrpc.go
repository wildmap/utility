package chanrpc

import (
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wildmap/utility/xlog"
)

// 全局会话ID计数器，用于生成递增的会话ID
var sessionIDCounter atomic.Int64

// nextSessionID 生成下一个会话ID
func nextSessionID() int64 {
	return sessionIDCounter.Add(1)
}

// Handler 方法句柄 处理CallInfo
type Handler func(ci *CallInfo)

// Callback 回调
type Callback func(ri *RetInfo)

// Server 代理服务器
type Server struct {
	functions map[uint32]Handler
	ChanCall  chan *CallInfo
	closed    atomic.Bool // 关闭标志
}

// CallInfo 调用参数
type CallInfo struct {
	id        uint32        // 消息类型id
	Req       any           // 入参
	chanRet   chan *RetInfo // 结果信息返回通道
	cb        Callback      // 回调
	hasRet    atomic.Bool   // 是否已经返回 由被调用方使用
	sessionID int64         // 会话ID
}

// Ret 调用请求的回调
func (ci *CallInfo) Ret(ret any) {
	// 检查回调是否已经使用过
	if !ci.hasRet.CompareAndSwap(false, true) {
		xlog.Warnf("chanrpc msg_id %d session_id %d can not ret twice, %s", ci.id, ci.sessionID, string(debug.Stack()))
		return
	}

	// 封装参数 执行回调
	if err := ci.ret(&RetInfo{Ack: ret}); err != nil {
		xlog.Warnf("chanrpc msg_id %d session_id %d ret error %v", ci.id, ci.sessionID, err)
	}
}

// RetWithError 带错误的返回
func (ci *CallInfo) RetWithError(ret any, e error) {
	// 检查回调是否已经使用过
	if !ci.hasRet.CompareAndSwap(false, true) {
		xlog.Warnf("chanrpc msg_id %d session_id %d can not ret twice %s", ci.id, ci.sessionID, string(debug.Stack()))
		return
	}
	// 封装参数 执行回调
	if err := ci.ret(&RetInfo{Ack: ret, Err: e}); err != nil {
		xlog.Warnf("chanrpc msg_id %d session_id %d ret error %v", ci.id, ci.sessionID, err)
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
			xlog.Errorf("chanrpc msg_id %d session_id %d ret err %v\n%s", ci.id, ci.sessionID, err, string(debug.Stack()))
		}
	}()
	// 封装参数 将结果信息放入返回通道
	ri.cb = ci.cb
	ri.clientSessionID = ci.sessionID

	// 使用select防止阻塞
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case ci.chanRet <- ri:
		return nil
	case <-timer.C:
		return fmt.Errorf("chanrpc ret timeout session_id %d", ci.sessionID)
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
	return MsgID(ri.Ack)
}

// Client 客户端
type Client struct {
	chanCall        chan *CallInfo   // 调用信息通道
	ChanASyncRet    chan *RetInfo    // 异步调用结果通道
	pendingASynCall atomic.Int64     // 最大待处理异步调用
	callList        map[int64]string // 待处理异步调用
	callListMu      sync.Mutex       // callList的互斥锁
	closed          atomic.Int64     // 关闭标志
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
	if msg == nil {
		return errors.New("chanrpc register: msg cannot be nil")
	}
	if f == nil {
		return errors.New("chanrpc register: handler cannot be nil")
	}
	msgID := MsgID(msg)
	if msgID <= 0 {
		return fmt.Errorf("chanrpc register: invalid msg type %v", reflect.TypeOf(msg))
	}

	if _, ok := s.functions[msgID]; ok {
		return fmt.Errorf("function ID %v: already registered, type: %v", msgID, reflect.TypeOf(msg))
	}
	xlog.Infof("chanrpc register: %v function ID %v", reflect.TypeOf(msg), msgID)
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
			xlog.Errorf("chanrpc msg_id %d session_id %d exec panic %v\n%s", ci.id, ci.sessionID, err, string(debug.Stack()))

			// 如果还没有返回，则返回错误
			if ci.hasRet.CompareAndSwap(false, true) {
				_ = ci.ret(&RetInfo{Err: err})
			}
		}
	}()

	// 根据id取handler
	handler, ok := s.functions[ci.id]
	if !ok {
		return fmt.Errorf("msg id %d session_id %d not registered, type: %T", ci.id, ci.sessionID, ci.Req)
	}

	handler(ci)
	return nil
}

// Exec 执行
func (s *Server) Exec(ci *CallInfo) {
	if ci == nil {
		xlog.Warnf("chanrpc exec: CallInfo is nil")
		return
	}
	ci.hasRet.Store(false)
	if err := s.exec(ci); err != nil {
		xlog.Warnln(err)
		ci.RetWithError(nil, err)
	}
}

// ExecDirect 直接执行请求（用于同协程内递归调用，避免死锁）
func (s *Server) ExecDirect(req any) *RetInfo {
	if s.IsClosed() {
		return &RetInfo{Err: errors.New("chanrpc server closed")}
	}

	id := MsgID(req)
	if id <= 0 {
		return &RetInfo{Err: errors.New("invalid message type")}
	}

	handler, ok := s.functions[id]
	if !ok {
		return &RetInfo{Err: fmt.Errorf("msg id %d not registered, type: %T", id, req)}
	}

	// 生成唯一会话ID
	sessionID := nextSessionID()

	// 创建返回通道
	chanRet := make(chan *RetInfo, 1)
	ci := &CallInfo{
		id:        id,
		Req:       req,
		chanRet:   chanRet,
		sessionID: sessionID,
	}

	// 捕获 panic
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic: %v\n%s", r, string(debug.Stack()))
			xlog.Errorf("chanrpc exec direct panic msg_id %d session_id %d err %v", id, sessionID, err)
			// 尝试写入错误
			select {
			case chanRet <- &RetInfo{Err: err}:
			default:
			}
		}
	}()

	// 直接执行 Handler
	handler(ci)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	// 等待结果 (Handler 中必须调用 Ret 或 RetWithError)
	select {
	case ri := <-chanRet:
		return ri
	case <-timer.C: // 使用超时防止忘记 Ret 导致的永久阻塞
		return &RetInfo{Err: errors.New("chanrpc exec direct timeout (handler forgot to ret?)")}
	}
}

// IsClosed 检查服务器是否已关闭
func (s *Server) IsClosed() bool {
	return s.closed.Load()
}

// Cast 直接投递消息 忽略任何错误和返回值
func (s *Server) Cast(req any) error {
	if s.IsClosed() {
		return errors.New("chanrpc server closed")
	}
	id := MsgID(req)
	if id <= 0 {
		return errors.New("invalid message type")
	}
	// 生成唯一会话ID用于追踪
	sessionID := nextSessionID()
	err := call(s.ChanCall, &CallInfo{
		id:        id,
		Req:       req,
		sessionID: sessionID,
	}, false)
	if err != nil {
		xlog.Warnf("chanrpc cast failed msg_id %d session_id %d err %v", id, sessionID, err)
	}
	return err
}

// Call 启动一个client来进行调用
func (s *Server) Call(req any) *RetInfo {
	return s.Open(0).Call(req)
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
		ChanASyncRet: make(chan *RetInfo, callLen),
		callList:     make(map[int64]string),
	}
	return c
}

// Attach 将client的请求通道依附于服务器
func (c *Client) Attach(s *Server) error {
	if s == nil {
		return errors.New("chanrpc attach: server cannot be nil")
	}
	if s.IsClosed() {
		return errors.New("chanrpc attach: server closed")
	}
	c.chanCall = s.ChanCall
	return nil
}

// IsClosed 检查客户端是否已关闭
func (c *Client) IsClosed() bool {
	return c.closed.Load() == 1
}

// Call 同步有结果调用
func (c *Client) Call(req any) *RetInfo {
	if c.IsClosed() {
		return &RetInfo{Err: errors.New("chanrpc client closed")}
	}
	id := MsgID(req)
	if id <= 0 {
		return &RetInfo{Err: errors.New("invalid message type")}
	}

	// 生成唯一会话ID
	sessionID := nextSessionID()

	// 为每个Call创建独立的返回通道，避免并发调用时结果混乱
	chanRet := make(chan *RetInfo, 1)
	err := call(c.chanCall, &CallInfo{
		id:        id,
		Req:       req,
		chanRet:   chanRet,
		sessionID: sessionID,
	}, true)
	if err != nil {
		xlog.Warnf("chanrpc sync call failed msg_id %d session_id %d err %v", id, sessionID, err)
		return &RetInfo{Err: err}
	}

	ri := <-chanRet
	return ri
}

// ASynCall 异步调用
func (c *Client) ASynCall(req any, cb Callback) error {
	if c.IsClosed() {
		return errors.New("chanrpc client closed")
	}

	if cb == nil {
		return errors.New("callback cannot be nil")
	}

	id := MsgID(req)
	if id <= 0 {
		return errors.New("invalid message type")
	}

	// 生成唯一会话ID
	sessionID := nextSessionID()

	err := call(c.chanCall, &CallInfo{
		id:        id,
		Req:       req,
		chanRet:   c.ChanASyncRet,
		cb:        cb,
		sessionID: sessionID,
	}, false)
	if err != nil {
		xlog.Warnf("chanrpc async call failed msg_id %d session_id %d err %v", id, sessionID, err)
		return err
	}

	name := reflect.TypeOf(req).String()
	c.callListMu.Lock()
	c.callList[sessionID] = name
	c.callListMu.Unlock()

	// 递增待处理计数
	c.pendingASynCall.Add(1)
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
	// 递减待处理计数
	c.pendingASynCall.Add(-1)
	c.callListMu.Lock()
	delete(c.callList, ri.clientSessionID)
	c.callListMu.Unlock()
	execCb(ri)
}

// Close 关闭client
func (c *Client) Close() {
	// 设置关闭标志
	if !c.closed.CompareAndSwap(0, 1) {
		xlog.Warnf("chanrpc client already closed")
		return
	}

	pending := c.pendingASynCall.Load()
	xlog.Infof("closing chanrpc client pending_calls %d", pending)

	if pending == 0 {
		return
	}

	// 使用WaitGroup等待所有异步调用完成
	var wg sync.WaitGroup
	wg.Go(func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()

		for {
			// 检查是否还有待处理的调用
			if c.pendingASynCall.Load() <= 0 {
				return
			}

			select {
			case ret := <-c.ChanASyncRet:
				c.Cb(ret)
			case <-timer.C:
				// 超时后强制清理
				remaining := c.pendingASynCall.Load()
				xlog.Warnf("chanrpc client close timeout remaining_calls %d", remaining)

				// 打印剩余的调用信息
				c.callListMu.Lock()
				for sid, _call := range c.callList {
					xlog.Warnf("pending call not finished session_id %d call %s", sid, _call)
				}
				c.callListMu.Unlock()

				// 强制清零
				c.pendingASynCall.Store(0)
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
	return c.pendingASynCall.Load() == 0
}

// PendingCount 获取待处理的异步调用数量
func (c *Client) PendingCount() int64 {
	return c.pendingASynCall.Load()
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
			err = fmt.Errorf("panic: %v\n%s", r, string(debug.Stack()))
			xlog.Warnf("chanrpc call panic msg_id %d session_id %d err %v", ci.id, ci.sessionID, err)
			if ci.chanRet != nil {
				// 使用 select 避免阻塞
				select {
				case ci.chanRet <- &RetInfo{Err: err, clientSessionID: ci.sessionID}:
				default:
					// 回调通道也满了或关闭了，忽略
				}
			}
		}
	}()

	if block {
		// 阻塞模式，添加超时控制
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case chanCall <- ci:
			return nil
		case <-timer.C:
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
