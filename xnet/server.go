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

// MaxMsgLen 单条消息的最大字节数限制（50MB）。
//
// 超过此限制的消息将被拒绝，防止恶意客户端发送超大消息耗尽服务器内存。
const MaxMsgLen = 50 * 1024 * 1024

// Server 多协议并发网络服务器，同时支持 TCP、KCP 和 WebSocket 连接。
//
// 架构设计：
//   - TCP 和 WebSocket 共享同一监听端口，通过 cmux 按协议特征分流
//   - KCP 监听同一地址的 UDP 端口
//   - 每个连接独立分配一个 goroutine 执行 IAgent.Run
//   - 通过 connCount 原子计数器和 conns map 追踪活跃连接
//
// 并发安全说明：
//   - conns 通过 mutexConns 互斥锁保护
//   - connCount 使用 atomic.Int32，保证并发读写安全
//   - started 使用 atomic.Bool，保证 Start/Stop 的幂等性
type Server struct {
	wg         sync.WaitGroup          // 追踪所有活跃 goroutine，确保 Stop 能等待其退出
	ctx        context.Context         // 服务器生命周期上下文，取消时停止接受新连接
	cancel     context.CancelFunc      // 取消函数，在 Stop 时调用
	addr       string                  // 监听地址，格式：[protocol://]host:port
	newAgent   func(conn IConn) IAgent // 连接代理工厂函数，每个新连接调用一次
	lns        []net.Listener          // 所有监听器列表，Stop 时统一关闭
	conns      map[IConn]struct{}      // 活跃连接集合，用于 Stop 时强制关闭
	mutexConns sync.Mutex              // 保护 conns 的并发读写
	connCount  atomic.Int32            // 当前活跃连接数（原子操作）
	started    atomic.Bool             // 服务器启动状态标志（防止重复启动/停止）
}

// NewServer 创建多协议网络服务器实例。
//
// addr 支持多种格式：
//   - ":8080"（TCP 默认）
//   - "tcp://:8080"
//   - "unix:///tmp/app.sock"
//
// newAgent 为每个新连接创建独立的 IAgent 实例，负责该连接的业务处理。
func NewServer(addr string, newAgent func(conn IConn) IAgent) *Server {
	return &Server{
		addr:     addr,
		newAgent: newAgent,
		conns:    make(map[IConn]struct{}),
	}
}

// validate 在启动前校验服务器配置的必要参数。
func (s *Server) validate() error {
	if s.addr == "" {
		return errors.New("addr is empty")
	}

	if s.newAgent == nil {
		return errors.New("NewAgent must not be nil")
	}

	return nil
}

// Start 初始化并启动所有协议的监听器，非阻塞返回。
//
// 启动流程：
//  1. 参数校验
//  2. 初始化生命周期上下文
//  3. 启动 TCP/Unix Socket 监听（TCP + WebSocket 共享端口，通过 cmux 分流）
//  4. 启动 KCP（UDP）监听
//  5. 启动 Unix Pipe 监听（本地进程间通信）
//
// 使用 CompareAndSwap 防止重复启动，确保幂等性。
func (s *Server) Start() error {
	err := s.validate()
	if err != nil {
		return fmt.Errorf("validate failed: %w", err)
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())

	err = s.listenTcpOrPipe(s.addr)
	if err != nil {
		xlog.Errorf("server failed to start: %v", err)
		return err
	}
	err = s.listenKcp(s.addr)
	if err != nil {
		xlog.Errorf("server failed to start: %v", err)
		return err
	}

	// 启动本地命名管道监听，供同机器进程间通信使用（如管理工具与服务器通信）
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

// listenKcp 创建并启动 KCP（基于 UDP 的可靠传输）监听器。
//
// KCP 特点：比 TCP 具有更低的延迟，适合游戏等对实时性要求高的场景，
// 代价是更高的 CPU 和带宽消耗。
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

// listenTcpOrPipe 创建 TCP 或 Unix Pipe 监听器，并通过 cmux 将同一端口的
// WebSocket（HTTP）和原始 TCP 流量分发到不同的处理器。
//
// cmux 工作原理：通过读取连接的前几个字节判断协议类型，
// HTTP 请求以 "GET/POST/..." 等方法开头，原始 TCP 则走默认匹配。
// SetReadTimeout(5s) 防止协议探测时永久阻塞。
func (s *Server) listenTcpOrPipe(addr string) error {
	ln, err := s.newTcpListener(s.ctx, addr)
	if err != nil {
		return fmt.Errorf("create tcp listener failed: %w", err)
	}

	s.lns = append(s.lns, ln)
	cmuxSvr := cmux.New(ln)
	cmuxSvr.SetReadTimeout(5 * time.Second) // 防止慢速客户端的协议探测阶段阻塞监听循环

	// 按优先级匹配：HTTP1 协议（WebSocket 升级请求）优先
	wsLn := cmuxSvr.Match(cmux.HTTP1Fast())
	s.lns = append(s.lns, wsLn)

	// 剩余所有连接作为原始 TCP Socket 处理
	socketLn := cmuxSvr.Match(cmux.Any())
	s.lns = append(s.lns, socketLn)

	s.wg.Go(func() {
		xlog.Infof("websocket server listening at %s", ln.Addr())
		s.startWebsocketServer(wsLn)
	})
	s.wg.Go(func() {
		xlog.Infof("tcp server listening at %s", ln.Addr())
		s.startSocketServer(socketLn)
	})
	s.wg.Go(func() {
		_ = cmuxSvr.Serve() // cmux 内部调度循环，阻塞直到监听器关闭
	})
	return nil
}

// newTcpListener 创建 TCP 或 Unix Socket 监听器，并用 ProxyProtocol 包装以支持 HAProxy。
//
// ProxyProtocol 支持 HAProxy 等负载均衡器透传真实客户端 IP，
// 使服务器能获取到 NAT 之前的原始 IP 地址。
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
	// 用 ProxyProtocol 包装，透明支持 PROXY protocol v1/v2
	ln = &proxyproto.Listener{Listener: ln}
	return ln, nil
}

// Stop 优雅关闭服务器：停止接受新连接 → 关闭所有活跃连接 → 等待所有 goroutine 退出。
//
// 关闭顺序：
//  1. 取消上下文，通知所有接受循环停止
//  2. 关闭所有监听器，使 Accept 立即返回错误
//  3. 关闭所有已建立的连接，使 ReadMsg 立即返回错误
//  4. 等待所有 goroutine 退出（wg.Wait）
//
// 使用 CompareAndSwap 保证 Stop 的幂等性。
func (s *Server) Stop() {
	if !s.started.CompareAndSwap(true, false) {
		xlog.Warnf("server already stopped")
		return
	}

	xlog.Infof("shutting down server...")

	if s.cancel != nil {
		s.cancel()
	}

	for _, ln := range s.lns {
		_ = ln.Close()
	}

	// 强制关闭所有存量连接，使其 IAgent.Run 中的 ReadMsg 立即返回 error
	for conn := range s.conns {
		if conn == nil {
			continue
		}
		_ = conn.Close()
	}

	s.wg.Wait()

	xlog.Infof("server stopped")
}

// GetConnCount 返回当前活跃连接数，可用于监控和限流判断。
func (s *Server) GetConnCount() int32 {
	return s.connCount.Load()
}

// handleConn 处理单个新连接的完整生命周期：创建 Agent → 初始化 → 注册 → 启动循环 → 清理。
//
// 每个连接在独立的 goroutine 中执行 Run，通过 wg 追踪，
// 保证所有连接在 Stop 时都能完成清理后再退出进程。
// Panic 恢复机制防止业务代码异常导致 goroutine 泄漏和服务器崩溃。
func (s *Server) handleConn(conn IConn) {
	agent := s.newAgent(conn)
	if agent == nil {
		_ = conn.Close()
		return
	}

	if err := agent.OnInit(s.ctx); err != nil {
		agent.OnClose(s.ctx)
		_ = conn.Close()
		xlog.Errorf("%s agent OnInit error: %v", conn.RemoteAddr(), err)
		return
	}

	// 注册连接到活跃连接集合，用于 Stop 时批量强制关闭
	s.mutexConns.Lock()
	s.conns[conn] = struct{}{}
	s.mutexConns.Unlock()
	s.connCount.Add(1)

	s.wg.Go(func() {
		defer func() {
			if rr := recover(); rr != nil {
				xlog.Errorf("%s agent panic, error: %v\n %s", conn.RemoteAddr(), rr, string(debug.Stack()))
			}

			agent.OnClose(s.ctx)
			_ = conn.Close()

			// 从活跃连接集合中注销，释放内存，避免 conns 无限增长
			s.mutexConns.Lock()
			delete(s.conns, conn)
			s.mutexConns.Unlock()

			s.connCount.Add(-1)
		}()

		agent.Run(s.ctx)
	})
}
