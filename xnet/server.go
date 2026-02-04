package xnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pires/go-proxyproto"
	"github.com/soheilhy/cmux"
	"github.com/xtaci/kcp-go"

	"github.com/wildmap/utility/xlog"
	"github.com/wildmap/utility/xnet/listeners"
	"github.com/wildmap/utility/xnet/sockets"
)

// MaxMsgLen 定义允许的最大消息大小(50MB)
const MaxMsgLen = 50 * 1024 * 1024

// Server 实现支持普通Socket和WebSocket连接的TCP&UDP服务器
// 使用连接多路复用(cmux)根据协议路由连接
// 特性:
// - 并发连接处理
// - 优雅关闭
// - 连接跟踪和限制
// - 自定义代理工厂处理连接
type Server struct {
	wg         sync.WaitGroup          // WaitGroup用于跟踪活动的goroutine
	ctx        context.Context         // 用于关闭信号的上下文
	cancel     context.CancelFunc      // 上下文的取消函数
	addr       string                  // 监听地址
	newAgent   func(conn IConn) IAgent // 创建代理的工厂函数
	lns        []net.Listener          // 监听器列表
	conns      map[IConn]struct{}      // 活动连接集合
	mutexConns sync.Mutex              // 用于跟踪连接的互斥锁
	connCount  atomic.Int32            // 活动连接的原子计数器
	started    atomic.Bool             // 服务器是否已启动
}

// NewServer 创建一个新的TCP服务器实例
// 参数: addr - 监听地址, newAgent - 创建代理的工厂函数
// 返回: Server实例指针
func NewServer(addr string, newAgent func(conn IConn) IAgent) *Server {
	return &Server{
		addr:     addr,
		newAgent: newAgent,
		conns:    make(map[IConn]struct{}),
	}
}

// validate 检查服务器配置是否有效
// 返回: 配置无效时的错误
func (s *Server) validate() error {
	if s.addr == "" {
		return errors.New("addr is empty")
	}

	if s.newAgent == nil {
		return errors.New("NewAgent must not be nil")
	}

	return nil
}

// Start 初始化并启动TCP服务器
// 该方法会阻塞直到服务器停止或遇到错误
//
// 启动流程:
// 1. 验证服务器配置
// 2. 初始化上下文和连接跟踪
// 3. 创建主监听器
// 4. 为socket和WebSocket设置连接多路复用器
// 5. 在独立的goroutine中启动socket和WebSocket处理器
// 6. 开始接受连接(阻塞)
// 返回: 启动失败时的错误
func (s *Server) Start() error {
	// 验证配置
	err := s.validate()
	if err != nil {
		return fmt.Errorf("validate failed: %w", err)
	}

	// 初始化用于关闭控制的上下文
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// 启动TCP监听器
	err = s.listenTcpOrPipe(s.addr)
	if err != nil {
		xlog.Errorf("server failed to start: %v", err)
		return err
	}
	// 启动KCP监听器
	err = s.listenKcp(s.addr)
	if err != nil {
		xlog.Errorf("server failed to start: %v", err)
		return err
	}

	// 启动管道监听器
	err = s.listenTcpOrPipe(sockets.ListenPipePath(filepath.Base(os.Args[0])))
	if err != nil {
		xlog.Errorf("server failed to start: %v", err)
		return err
	}

	if !s.started.CompareAndSwap(false, true) {
		return errors.New("server already started")
	}

	return nil
}

func (s *Server) listenKcp(addr string) error {
	ln, err := kcp.Listen(addr)
	if err != nil {
		return fmt.Errorf("create kcp listener failed: %w", err)
	}

	s.lns = append(s.lns, ln)
	s.wg.Go(func() {
		xlog.Infof("kcp server listening at %s", ln.Addr())
		s.startSocketServer(ln)
	})
	return nil
}

func (s *Server) listenTcpOrPipe(addr string) error {
	// 创建主监听器
	ln, err := s.newTcpListener(s.ctx, addr)
	if err != nil {
		return fmt.Errorf("create tcp listener failed: %w", err)
	}

	s.lns = append(s.lns, ln)
	// 设置连接多路复用器
	cmuxSvr := cmux.New(ln)
	cmuxSvr.SetReadTimeout(5 * time.Second)

	// 匹配HTTP/WebSocket连接
	wsLn := cmuxSvr.Match(cmux.HTTP1Fast())
	s.lns = append(s.lns, wsLn)

	// 匹配所有其他连接
	socketLn := cmuxSvr.Match(cmux.Any())
	s.lns = append(s.lns, socketLn)

	// 启动WebSocket和Socket处理器
	s.wg.Go(func() {
		xlog.Infof("websocket server listening at %s", ln.Addr())
		s.startWebsocketServer(wsLn)
	})
	s.wg.Go(func() {
		xlog.Infof("tcp server listening at %s", ln.Addr())
		s.startSocketServer(socketLn)
	})
	s.wg.Go(func() {
		_ = cmuxSvr.Serve()
	})
	return nil
}

func (s *Server) newTcpListener(ctx context.Context, addr string) (net.Listener, error) {
	proto, host, found := strings.Cut(addr, "://")
	if !found {
		proto = "tcp"
		host = addr
	}

	ln, err := listeners.New(ctx, proto, host, nil)
	if err != nil {
		return nil, err
	}
	// 用ProxyProtocol包装以支持HAProxy PROXY协议
	ln = &proxyproto.Listener{Listener: ln}
	return ln, nil
}

// Stop 优雅地关闭服务器
// 关闭流程:
// 1. 取消上下文以停止接受新连接
// 2. 关闭主监听器
// 3. 关闭所有活动连接
// 4. 等待所有goroutine完成
func (s *Server) Stop() {
	if !s.started.CompareAndSwap(true, false) {
		xlog.Warnf("server already stopped")
		return
	}

	xlog.Infof("shutting down server...")

	// 取消上下文以停止接受新连接
	if s.cancel != nil {
		s.cancel()
	}

	for _, ln := range s.lns {
		_ = ln.Close()
	}

	// 关闭所有活动连接
	for conn := range s.conns {
		if conn == nil {
			continue
		}
		_ = conn.Close()
	}

	// 等待所有goroutine完成
	s.wg.Wait()

	xlog.Infof("server stopped")
}

// GetConnCount 返回当前活动连接数
func (s *Server) GetConnCount() int32 {
	return s.connCount.Load()
}

// handleConn 处理新连接,执行以下步骤:
// 1. 使用工厂函数创建代理
// 2. 初始化代理
// 3. 注册连接
// 4. 在goroutine中启动代理的处理循环
// 5. 代理完成时进行清理
//
// 该方法同时用于socket和WebSocket连接
// 参数: conn - 网络连接接口
func (s *Server) handleConn(conn IConn) {
	// 为此连接创建代理
	agent := s.newAgent(conn)
	if agent == nil {
		_ = conn.Close()
		return
	}

	// 初始化代理
	if err := agent.OnInit(s.ctx); err != nil {
		agent.OnClose(s.ctx)
		_ = conn.Close()
		xlog.Errorf("%s agent OnInit error: %v", conn.RemoteAddr(), err)
		return
	}

	// 注册连接
	s.mutexConns.Lock()
	s.conns[conn] = struct{}{}
	s.mutexConns.Unlock()
	// 增加连接计数器
	s.connCount.Add(1)

	// 在goroutine中启动代理处理
	s.wg.Go(func() {
		defer func() {
			// 从panic中恢复
			if rr := recover(); rr != nil {
				xlog.Errorf("%s agent panic, error: %v\n %s", conn.RemoteAddr(), rr, string(debug.Stack()))
			}

			// 清理代理
			agent.OnClose(s.ctx)

			// 关闭连接
			_ = conn.Close()

			// 注销连接
			s.mutexConns.Lock()
			delete(s.conns, conn)
			s.mutexConns.Unlock()

			// 减少连接计数器
			s.connCount.Add(-1)
		}()

		// 运行代理的主处理循环
		agent.Run(s.ctx)
	})
}
