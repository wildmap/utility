package chanrpc

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wildmap/utility/xlog"
)

// Client 客户端
type Client struct {
	ChanAsyncRet     chan *RetInfo // 异步调用结果通道
	pendingAsyncCall atomic.Int64  // 最大待处理异步调用
	closed           atomic.Bool   // 关闭标志
}

// NewClient 新建客户端 设置异步结果通道的最大缓冲值
func NewClient(callLen int) *Client {
	c := &Client{
		ChanAsyncRet: make(chan *RetInfo, callLen),
	}
	return c
}

// IsClosed 检查客户端是否已关闭
func (c *Client) IsClosed() bool {
	return c.closed.Load()
}

// check 检查调用参数
func (c *Client) check(s *Server, request any) (uint32, error) {
	if s == nil {
		return 0, ErrServerNil
	}
	if s.IsClosed() {
		return 0, ErrServerClosed
	}
	if c.IsClosed() {
		return 0, ErrClientClosed
	}
	messageID := MessageID(request)
	if messageID <= 0 {
		return 0, ErrInvalidMsgType
	}
	return messageID, nil
}

// Call 同步调用指定Server
func (c *Client) Call(s *Server, request any) *RetInfo {
	messageID, err := c.check(s, request)
	if err != nil {
		xlog.Warnf("chanrpc sync call failed message_id %d err %v", messageID, err)
		return &RetInfo{Err: err}
	}

	// 为每个Call创建独立的返回通道
	chanRet := make(chan *RetInfo, 1)
	err = c.call(s.ChanCall, &CallInfo{
		messageID: messageID,
		Request:   request,
		chanRet:   chanRet,
	}, true)
	if err != nil {
		xlog.Warnf("chanrpc sync call failed message_id %d err %v", messageID, err)
		return &RetInfo{Err: err}
	}

	ri := <-chanRet
	return ri
}

// AsyncCall 异步调用指定Server
func (c *Client) AsyncCall(s *Server, request any, callback Callback) error {
	if callback == nil {
		return ErrCallbackNil
	}

	messageID, err := c.check(s, request)
	if err != nil {
		xlog.Warnf("chanrpc async call failed message_id %d err %v", messageID, err)
		return err
	}

	err = c.call(s.ChanCall, &CallInfo{
		messageID: messageID,
		Request:   request,
		chanRet:   c.ChanAsyncRet,
		callback:  callback,
	}, false)
	if err != nil {
		xlog.Warnf("chanrpc async call failed message_id %d err %v", messageID, err)
		return err
	}

	// 递增待处理计数
	c.pendingAsyncCall.Add(1)
	return nil
}

// Cast 直接投递消息
func (c *Client) Cast(s *Server, request any) {
	messageID, err := c.check(s, request)
	if err != nil {
		xlog.Warnf("chanrpc cast failed message_id %d err %v", messageID, err)
		return
	}

	err = c.call(s.ChanCall, &CallInfo{
		messageID: messageID,
		Request:   request,
	}, false)
	if err != nil {
		xlog.Warnf("chanrpc cast failed message_id %d err %v", messageID, err)
	}
}

// execCallback 执行回调
func (c *Client) execCallback(ri *RetInfo) {
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("chanrpc callback panic: %v\n%s", r, string(debug.Stack()))
		}
	}()

	if ri.callback != nil {
		ri.callback(ri)
	}
}

// AsyncCallback 执行回调
func (c *Client) AsyncCallback(ri *RetInfo) {
	// 递减待处理计数
	c.pendingAsyncCall.Add(-1)
	c.execCallback(ri)
}

// Close 关闭client
func (c *Client) Close() {
	// 设置关闭标志
	if !c.closed.CompareAndSwap(false, true) {
		xlog.Warnf("chanrpc client already closed")
		return
	}

	pending := c.pendingAsyncCall.Load()
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
			if c.pendingAsyncCall.Load() <= 0 {
				return
			}

			select {
			case ret := <-c.ChanAsyncRet:
				c.AsyncCallback(ret)
			case <-timer.C:
				// 超时后强制清理
				remaining := c.pendingAsyncCall.Load()
				xlog.Warnf("chanrpc client close timeout remaining_calls %d", remaining)

				// 强制清零
				c.pendingAsyncCall.Store(0)
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
	return c.pendingAsyncCall.Load() == 0
}

// PendingCount 获取待处理的异步调用数量
func (c *Client) PendingCount() int64 {
	return c.pendingAsyncCall.Load()
}

// call 调用,分阻塞与非阻塞模式,仅仅将请求放入请求通道
func (c *Client) call(chanCall chan *CallInfo, ci *CallInfo, block bool) (err error) {
	if chanCall == nil {
		return ErrCallChannelNil
	}
	if ci == nil {
		return ErrCallInfoNil
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v\n%s", r, string(debug.Stack()))
			xlog.Warnf("chanrpc call panic message_id %d err %v", ci.MessageID(), err)
			if ci.chanRet != nil {
				// 使用 select 避免阻塞
				select {
				case ci.chanRet <- &RetInfo{Err: err}:
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
			return ErrCallTimeout
		}
	}

	// 非阻塞模式
	select {
	case chanCall <- ci:
		return nil
	default:
		reqType := "unknown"
		if ci.Request != nil {
			reqType = reflect.TypeOf(ci.Request).String()
		}
		return fmt.Errorf("server chanrpc channel full, msg %v %+v", reqType, ci.Request)
	}
}
