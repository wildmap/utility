package chanrpc

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// 测试用的消息结构体
type TestReq struct {
	Value int
}

type TestAck struct {
	Result int
}

type TestReq2 struct {
	Name string
}

type TestAck2 struct {
	Greeting string
}

// 实现IMsgID接口的消息
type CustomMsgIDReq struct {
	Data string
}

func (c *CustomMsgIDReq) MsgID() uint32 {
	return 99999
}

// TestMsgID 测试消息ID生成
func TestMsgID(t *testing.T) {
	tests := []struct {
		name string
		msg  any
		want uint32
	}{
		{
			name: "nil message",
			msg:  nil,
			want: 0,
		},
		{
			name: "struct pointer",
			msg:  &TestReq{},
			want: MsgID(&TestReq{}),
		},
		{
			name: "struct value",
			msg:  TestReq{},
			want: MsgID(TestReq{}),
		},
		{
			name: "custom msg id",
			msg:  &CustomMsgIDReq{},
			want: 99999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MsgID(tt.msg)
			if got != tt.want {
				t.Errorf("MsgID() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestServerNew 测试服务器创建
func TestServerNew(t *testing.T) {
	s := NewServer(10000)
	t.Run("server", func(t *testing.T) {
		if s.ChanCall == nil {
			t.Error("Server.ChanCall is nil")
		}
		if s.functions == nil {
			t.Error("Server.functions is nil")
		}
	})
}

// TestServerRegister 测试服务器注册
func TestServerRegister(t *testing.T) {
	s := NewServer(10000)
	t.Run("normal register", func(t *testing.T) {
		handler := func(ci *CallInfo) {
			ci.Ret(&TestAck{Result: 100})
		}
		err := s.Register(&TestReq{}, handler)
		if err != nil {
			t.Errorf("Register error %v", err)
		}
	})

	t.Run("duplicate register should return error", func(t *testing.T) {
		handler := func(ci *CallInfo) {}
		err := s.Register(&TestReq{}, handler)
		if err == nil {
			t.Error("Expected error for duplicate registration")
		}
	})

	t.Run("nil message should return error", func(t *testing.T) {
		handler := func(ci *CallInfo) {}
		err := s.Register(nil, handler)
		if err == nil {
			t.Error("Expected error for nil message")
		}
	})

	t.Run("nil handler should return error", func(t *testing.T) {
		err := s.Register(&TestReq2{}, nil)
		if err == nil {
			t.Error("Expected error for nil handler")
		}
	})
}

// TestServerCast 测试Cast调用
func TestServerCast(t *testing.T) {
	s := NewServer(10000)
	received := make(chan bool, 1)

	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		received <- true
	})
	if err != nil {
		t.Errorf("Register error %v", err)
	}

	err = s.Cast(&TestReq{Value: 10})
	if err != nil {
		t.Errorf("Cast() error = %v", err)
	}

	// 处理消息
	go func() {
		for ci := range s.ChanCall {
			s.Exec(ci)
		}
	}()

	select {
	case <-received:
		// 成功接收
	case <-time.After(1 * time.Second):
		t.Error("Cast message not received")
	}
}

// TestServerCall 测试同步调用
func TestServerCall(t *testing.T) {
	s := NewServer(10000)

	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		req := ci.Req.(*TestReq)
		ci.Ret(&TestAck{Result: req.Value * 2})
	})
	if err != nil {
		t.Errorf("Register error %v", err)
	}

	// 启动服务器处理循环
	go func() {
		for ci := range s.ChanCall {
			s.Exec(ci)
		}
	}()

	ri := s.Call(&TestReq{Value: 10})
	if ri.Err != nil {
		t.Errorf("Call() error = %v", ri.Err)
	}
	if ack, ok := ri.Ack.(*TestAck); ok {
		if ack.Result != 20 {
			t.Errorf("Call() result = %v, want %v", ack.Result, 20)
		}
	} else {
		t.Error("Call() returned wrong type")
	}
}

// TestClientNew 测试客户端创建
func TestClientNew(t *testing.T) {
	c := NewClient(10000)
	if c == nil {
		t.Error("NewClient() returned nil")
	}
	t.Run("client", func(t *testing.T) {
		if c.ChanASyncRet == nil {
			t.Error("Client.ChanAsynRet is nil")
		}
	})
}

// TestClientAttach 测试客户端附加
func TestClientAttach(t *testing.T) {
	s := NewServer(10000)
	c := NewClient(10000)

	t.Run("normal attach", func(t *testing.T) {
		if err := c.Attach(s); err != nil {
			t.Errorf("Attach() error = %v", err)
		}
		if c.chanCall != s.ChanCall {
			t.Error("Client not properly attached to server")
		}
	})

	t.Run("nil server should panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for nil server")
			}
		}()
		c2 := NewClient(10000)
		if err := c2.Attach(nil); err != nil {
			panic(err)
		}
	})
}

// TestClientSyncCall 测试同步调用
func TestClientSyncCall(t *testing.T) {
	s := NewServer(10000)
	c := s.Open(10000)

	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		req := ci.Req.(*TestReq)
		ci.Ret(&TestAck{Result: req.Value * 3})
	})
	if err != nil {
		t.Errorf("Register error %v", err)
	}

	// 启动服务器处理循环
	go func() {
		for ci := range s.ChanCall {
			s.Exec(ci)
		}
	}()

	t.Run("successful call", func(t *testing.T) {
		ri := c.Call(&TestReq{Value: 5})
		if ri.Err != nil {
			t.Errorf("Call() error = %v", ri.Err)
		}
		if ack, ok := ri.Ack.(*TestAck); ok {
			if ack.Result != 15 {
				t.Errorf("Call() result = %v, want %v", ack.Result, 15)
			}
		} else {
			t.Error("Call() returned wrong type")
		}
	})

	t.Run("call with error", func(t *testing.T) {
		err = s.Register(&TestReq2{}, func(ci *CallInfo) {
			ci.RetWithError(nil, errors.New("test error"))
		})
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		ri := c.Call(&TestReq2{Name: "test"})
		if ri.Err == nil {
			t.Error("Expected error but got nil")
		}
		if ri.Err.Error() != "test error" {
			t.Errorf("Error = %v, want %v", ri.Err, "test error")
		}
	})
}

// TestClientAsyncCall 测试异步调用
func TestClientAsyncCall(t *testing.T) {
	s := NewServer(10000)
	c := s.Open(10000)

	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		req := ci.Req.(*TestReq)
		ci.Ret(&TestAck{Result: req.Value * 4})
	})
	if err != nil {
		t.Errorf("Register error %v", err)
	}

	// 启动服务器处理循环
	go func() {
		for ci := range s.ChanCall {
			s.Exec(ci)
		}
	}()

	t.Run("successful async call", func(t *testing.T) {
		done := make(chan bool, 1)
		var result int

		err = c.ASynCall(&TestReq{Value: 6}, func(ri *RetInfo) {
			if ri.Err != nil {
				t.Errorf("Callback error = %v", ri.Err)
			}
			if ack, ok := ri.Ack.(*TestAck); ok {
				result = ack.Result
			}
			done <- true
		})

		if err != nil {
			t.Errorf("AsynCall() error = %v", err)
		}

		// 处理异步返回
		go func() {
			for ri := range c.ChanASyncRet {
				c.Cb(ri)
			}
		}()

		select {
		case <-done:
			if result != 24 {
				t.Errorf("Async result = %v, want %v", result, 24)
			}
		case <-time.After(2 * time.Second):
			t.Error("Async call timeout")
		}
	})

	t.Run("nil callback should error", func(t *testing.T) {
		err = c.ASynCall(&TestReq{Value: 1}, nil)
		if err == nil {
			t.Error("Expected error for nil callback")
		}
	})
}

// TestCallInfoRet 测试CallInfo的返回方法
func TestCallInfoRet(t *testing.T) {
	chanRet := make(chan *RetInfo, 1)

	t.Run("normal ret", func(t *testing.T) {
		ci := &CallInfo{
			id:      1,
			chanRet: chanRet,
		}

		ci.Ret(&TestAck{Result: 100})

		select {
		case ri := <-chanRet:
			if ack, ok := ri.Ack.(*TestAck); ok {
				if ack.Result != 100 {
					t.Errorf("Ret result = %v, want %v", ack.Result, 100)
				}
			}
		case <-time.After(1 * time.Second):
			t.Error("Ret timeout")
		}
	})

	t.Run("double ret should not panic", func(t *testing.T) {
		ci := &CallInfo{
			id:      2,
			chanRet: chanRet,
		}

		ci.Ret(&TestAck{Result: 100})
		ci.Ret(&TestAck{Result: 200}) // 第二次调用应该被忽略

		// 只应该收到一个结果
		select {
		case <-chanRet:
			// 第一个结果
		case <-time.After(100 * time.Millisecond):
			t.Error("Should receive first ret")
		}

		// 不应该有第二个结果
		select {
		case <-chanRet:
			t.Error("Should not receive second ret")
		case <-time.After(100 * time.Millisecond):
			// 正确，没有第二个结果
		}
	})
}

// TestClientClose 测试客户端关闭
func TestClientClose(t *testing.T) {
	s := NewServer(10000)
	c := s.Open(10000)

	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		time.Sleep(50 * time.Millisecond) // 模拟处理时间
		req := ci.Req.(*TestReq)
		ci.Ret(&TestAck{Result: req.Value})
	})
	if err != nil {
		t.Errorf("Register error %v", err)
	}

	// 启动服务器处理循环
	go func() {
		for ci := range s.ChanCall {
			s.Exec(ci)
		}
	}()

	// 发起几个异步调用
	go func() {
		for ri := range c.ChanASyncRet {
			c.Cb(ri)
		}
	}()

	for i := 0; i < 3; i++ {
		err = c.ASynCall(&TestReq{Value: i}, func(ri *RetInfo) {})
		if err != nil {
			t.Errorf("AsynCall() error = %v", err)
		}
	}

	// 关闭客户端
	c.Close()

	if !c.IsClosed() {
		t.Error("Client should be closed")
	}

	if c.PendingCount() != 0 {
		t.Errorf("Pending count = %v, want 0", c.PendingCount())
	}
}

// TestServerClose 测试服务器关闭
func TestServerClose(t *testing.T) {
	s := NewServer(10000)

	// 注册处理器
	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		ci.Ret(&TestAck{Result: 100})
	})
	if err != nil {
		t.Errorf("Register error %v", err)
	}

	// 发送一些消息
	_ = s.Cast(&TestReq{Value: 1})
	_ = s.Cast(&TestReq{Value: 2})

	// 关闭服务器
	s.Close()

	// 尝试再次发送应该失败
	err = s.Cast(&TestReq{Value: 3})
	if err == nil {
		t.Error("Expected error when casting to closed server")
	}
}

// TestConcurrentCalls 测试并发调用
func TestConcurrentCalls(t *testing.T) {
	s := NewServer(10000)
	c := s.Open(10000)

	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		req := ci.Req.(*TestReq)
		ci.Ret(&TestAck{Result: req.Value})
	})
	if err != nil {
		t.Errorf("Register error %v", err)
	}

	// 启动服务器处理循环
	go func() {
		for ci := range s.ChanCall {
			s.Exec(ci)
		}
	}()

	// 并发同步调用
	t.Run("concurrent sync calls", func(t *testing.T) {
		var wg sync.WaitGroup
		callCount := 50

		for i := 0; i < callCount; i++ {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				ri := c.Call(&TestReq{Value: val})
				if ri.Err != nil {
					t.Errorf("Call() error = %v", ri.Err)
				}
			}(i)
		}

		wg.Wait()
	})

	// 并发异步调用
	t.Run("concurrent async calls", func(t *testing.T) {
		var wg sync.WaitGroup
		callCount := 50

		// 启动异步返回处理
		go func() {
			for ri := range c.ChanASyncRet {
				c.Cb(ri)
			}
		}()

		for i := 0; i < callCount; i++ {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				err = c.ASynCall(&TestReq{Value: val}, func(ri *RetInfo) {
					if ri.Err != nil {
						t.Errorf("Callback error = %v", ri.Err)
					}
				})
				if err != nil {
					t.Errorf("AsynCall() error = %v", err)
				}
			}(i)
		}

		wg.Wait()
		time.Sleep(500 * time.Millisecond) // 等待异步调用完成
	})
}

// TestPanicRecovery 测试panic恢复
func TestPanicRecovery(t *testing.T) {
	s := NewServer(10000)
	c := s.Open(10000)

	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		panic("test panic")
	})
	if err != nil {
		t.Errorf("Register error %v", err)
	}

	// 启动服务器处理循环
	go func() {
		for ci := range s.ChanCall {
			s.Exec(ci)
		}
	}()

	ri := c.Call(&TestReq{Value: 1})
	if ri.Err == nil {
		t.Error("Expected error from panic")
	}
}

// TestCastAndCallMixedUsage 测试Cast和Call混用时的结果残留问题
func TestCastAndCallMixedUsage(t *testing.T) {
	s := NewServer(10000)
	c := s.Open(10000)

	// 注册一个handler，模拟有返回值的处理
	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		req := ci.Req.(*TestReq)
		// 模拟处理延迟
		time.Sleep(10 * time.Millisecond)
		ci.Ret(&TestAck{Result: req.Value * 10})
	})
	if err != nil {
		t.Fatalf("Register error %v", err)
	}

	// 启动服务器处理循环
	go func() {
		for ci := range s.ChanCall {
			s.Exec(ci)
		}
	}()

	t.Run("mixed cast and call should not cause confusion", func(t *testing.T) {
		// 先使用Cast发送消息（无需返回值）
		err := s.Cast(&TestReq{Value: 1})
		if err != nil {
			t.Errorf("Cast error: %v", err)
		}

		// 立即使用Call发送消息（需要返回值）
		ri := c.Call(&TestReq{Value: 2})
		if ri.Err != nil {
			t.Errorf("Call error: %v", ri.Err)
		}

		// 检查Call的返回值是否正确
		if ack, ok := ri.Ack.(*TestAck); ok {
			if ack.Result != 20 { // 2 * 10 = 20
				t.Errorf("Call result = %v, want 20, got confused result", ack.Result)
			}
		} else {
			t.Error("Call returned wrong type")
		}
	})

	t.Run("concurrent shared client calls should not mix results", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan string, 10)

		// 使用同一个Client并发调用
		for i := 1; i <= 10; i++ {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				ri := c.Call(&TestReq{Value: val})
				if ri.Err != nil {
					errors <- ri.Err.Error()
					return
				}

				if ack, ok := ri.Ack.(*TestAck); ok {
					expected := val * 10
					if ack.Result != expected {
						errors <- "result mismatch"
					}
				} else {
					errors <- "wrong type"
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// 检查是否有错误
		for err := range errors {
			t.Errorf("Concurrent call error: %v", err)
		}
	})
}

// BenchmarkSyncCall 同步调用基准测试
func BenchmarkSyncCall(b *testing.B) {
	s := NewServer(10000)
	c := s.Open(10000)

	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		req := ci.Req.(*TestReq)
		ci.Ret(&TestAck{Result: req.Value})
	})
	if err != nil {
		b.Errorf("Register error %v", err)
	}

	go func() {
		for ci := range s.ChanCall {
			s.Exec(ci)
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Call(&TestReq{Value: i})
	}
}

// BenchmarkAsyncCall 异步调用基准测试
func BenchmarkAsyncCall(b *testing.B) {
	s := NewServer(10000)
	c := s.Open(10000)

	err := s.Register(&TestReq{}, func(ci *CallInfo) {
		req := ci.Req.(*TestReq)
		ci.Ret(&TestAck{Result: req.Value})
	})
	if err != nil {
		b.Errorf("Register error %v", err)
	}

	go func() {
		for ci := range s.ChanCall {
			s.Exec(ci)
		}
	}()

	go func() {
		for ri := range c.ChanASyncRet {
			c.Cb(ri)
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.ASynCall(&TestReq{Value: i}, func(ri *RetInfo) {})
	}
}
