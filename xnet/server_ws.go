package xnet

import (
	"context"
	"errors"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/soheilhy/cmux"

	"github.com/wildmap/utility/xlog"
)

// upgrader WebSocket 升级器配置。
//
// 关键配置说明：
//   - HandshakeTimeout(10s)：防止握手阶段的慢速攻击（slowloris）
//   - ReadBufferSize/WriteBufferSize(4KB)：平衡内存占用和 I/O 效率
//   - CheckOrigin 返回 true：不验证 Origin 头，服务器端的跨域控制应在业务层实现
var upgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   4096,
	WriteBufferSize:  4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// startWebsocketServer 以 HTTP 服务器形式运行 WebSocket 接受循环。
//
// 使用 net/http.Server 而非直接处理 TCP 连接，原因：
//   - 自动处理 HTTP/1.1 的 keep-alive 和请求路由
//   - 超时配置（Read/Write/Idle）防止连接长时间占用资源
//   - BaseContext 将服务器生命周期上下文注入每个请求的 context
func (s *Server) startWebsocketServer(ln net.Listener) {
	httpSvr := &http.Server{
		Handler:      http.HandlerFunc(s.handleWebsocketConn),
		ReadTimeout:  120 * time.Second, // 读取请求体的超时时间
		WriteTimeout: 120 * time.Second, // 写入响应的超时时间
		IdleTimeout:  120 * time.Second, // Keep-Alive 连接的空闲超时
		BaseContext: func(_ net.Listener) context.Context {
			return s.ctx // 将服务器上下文注入 HTTP 请求，ctx 取消时关闭连接
		},
	}

	if err := httpSvr.Serve(ln); err != nil &&
		!errors.Is(err, net.ErrClosed) &&
		!errors.Is(err, http.ErrServerClosed) &&
		!errors.Is(err, cmux.ErrServerClosed) &&
		!errors.Is(err, cmux.ErrListenerClosed) {
		xlog.Fatalln("Websocket server error: %v", err)
	}
}

// handleWebsocketConn 处理 HTTP 升级请求，将 HTTP 连接升级为 WebSocket 协议。
//
// 处理流程：
//  1. 检查服务器是否正在关闭（拒绝新连接）
//  2. 调用 upgrader.Upgrade 完成协议升级（内部处理 101 Switching Protocols）
//  3. 提取真实客户端 IP（支持反向代理场景）
//  4. 创建 WSConn 并委托给统一的连接处理逻辑
//
// Panic 恢复机制防止单个连接的异常影响整个 WebSocket 服务。
func (s *Server) handleWebsocketConn(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rr := recover(); rr != nil {
			xlog.Errorf("ws handler panic, error: %v\n%s", rr, string(debug.Stack()))
		}
	}()

	// 服务器关闭时拒绝新 WebSocket 连接，返回 503
	select {
	case <-s.ctx.Done():
		http.Error(w, "Server is shutting down", http.StatusServiceUnavailable)
		return
	default:
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade 失败时已向客户端写入 HTTP 错误响应，此处仅静默返回
		return
	}
	remoteAddr := s.newRealAddr(s.getHTTPClientIP(r))

	s.handleConn(NewWSConn(conn, remoteAddr))
}

// getHTTPClientIP 从 HTTP 请求中提取真实客户端 IP 地址。
//
// 提取优先级（从高到低）：
//  1. X-Forwarded-For 第一个 IP（最常见的反向代理头）
//  2. X-Real-IP（Nginx 常用的透传头）
//  3. Forwarded 标准头（RFC 7239，for= 字段）
//  4. RemoteAddr（直接连接时的 IP）
//
// 注意：X-Forwarded-For 可被客户端伪造，在信任内网代理的场景下使用才安全。
func (s *Server) getHTTPClientIP(r *http.Request) string {
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
	}

	// X-Forwarded-For：可包含多个 IP（client, proxy1, proxy2），取第一个
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// X-Real-IP：单个真实客户端 IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// Forwarded（RFC 7239）：格式为 for=IP; proto=...; by=...
	if forwarded := r.Header.Get("Forwarded"); forwarded != "" {
		for part := range strings.SplitSeq(forwarded, ";") {
			if after, ok := strings.CutPrefix(strings.TrimSpace(part), "for="); ok {
				ip := after
				ip = strings.Trim(ip, "\"")
				// 移除端口号（IPv4: host:port，IPv6: [::1]:port）
				if idx := strings.LastIndex(ip, ":"); idx != -1 {
					ip = ip[:idx]
				}
				ip = strings.Trim(ip, "[]") // 移除 IPv6 方括号
				if net.ParseIP(ip) != nil {
					return ip
				}
			}
		}
	}

	return remoteIP
}

// realAddr 实现 net.Addr 接口，用于将真实客户端 IP 封装为地址对象。
//
// WebSocket 连接通过 HTTP 升级，默认 RemoteAddr 是代理服务器的 IP，
// 通过此结构体替换为从请求头中提取的真实客户端 IP。
type realAddr struct {
	network string
	addr    string
}

// Network 返回网络协议类型。
func (r *realAddr) Network() string {
	return r.network
}

// String 返回地址字符串。
func (r *realAddr) String() string {
	return r.addr
}

// newRealAddr 创建 realAddr 实例，网络类型固定为 "tcp"。
func (s *Server) newRealAddr(addr string) net.Addr {
	return &realAddr{
		network: "tcp",
		addr:    addr,
	}
}
