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

// upgrader 配置WebSocket升级器:
// - 10秒握手超时
// - 4KB读写缓冲区
// - 宽松的来源检查(允许所有来源)
var upgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   4096,
	WriteBufferSize:  4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许来自任何来源的连接
	},
}

// startWebsocketServer 运行WebSocket服务器
func (s *Server) startWebsocketServer(ln net.Listener) {
	// 为WebSocket连接配置HTTP服务器
	httpSvr := &http.Server{
		Handler:      http.HandlerFunc(s.handleWebsocketConn),
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return s.ctx
		},
	}

	// 开始服务(阻塞)
	if err := httpSvr.Serve(ln); err != nil &&
		!errors.Is(err, net.ErrClosed) &&
		!errors.Is(err, http.ErrServerClosed) &&
		!errors.Is(err, cmux.ErrServerClosed) &&
		!errors.Is(err, cmux.ErrListenerClosed) {
		xlog.Fatalln("Websocket server error: %v", err)
	}
}

// handleWebsocketConn 处理HTTP请求并将其升级为WebSocket连接
// 执行以下步骤:
// 1. 检查服务器是否正在关闭
// 2. 将HTTP连接升级为WebSocket
// 3. 封装连接并委托给通用处理器
//
// 从panic中恢复以防止服务器崩溃
func (s *Server) handleWebsocketConn(w http.ResponseWriter, r *http.Request) {
	defer func() {
		// 从panic中恢复
		if rr := recover(); rr != nil {
			xlog.Errorf("ws handler panic, error: %v\n%s", rr, string(debug.Stack()))
		}
	}()

	// 检查服务器是否正在关闭
	select {
	case <-s.ctx.Done():
		http.Error(w, "Server is shutting down", http.StatusServiceUnavailable)
		return
	default:
	}

	// 将HTTP连接升级为WebSocket协议
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade已向客户端发送错误响应
		return
	}
	// 提取真实客户端IP
	remoteAddr := s.newRealAddr(s.getHTTPClientIP(r))

	// 处理WebSocket连接
	s.handleConn(NewWSConn(conn, remoteAddr))
}

// getHTTPClientIP 从HTTP请求中获取客户端真实IP
// 优先级:
// 1. 从X-Forwarded-For获取第一个IP(优先)
// 2. 从X-Real-IP获取(备选)
// 3. 从Forwarded标准头获取(RFC 7239)
// 4. 直接使用RemoteAddr
// 参数: r - HTTP请求对象
// 返回: 客户端IP地址字符串
func (s *Server) getHTTPClientIP(r *http.Request) string {
	// 获取RemoteAddr的IP部分
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
	}

	// 从X-Forwarded-For获取(优先)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For可能包含多个IP,格式: client, proxy1, proxy2
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// 从X-Real-IP获取(备选)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// 从Forwarded标准头获取(RFC 7239)
	if forwarded := r.Header.Get("Forwarded"); forwarded != "" {
		// 格式示例: for=192.0.2.60;proto=http;by=203.0.113.43
		for _, part := range strings.Split(forwarded, ";") {
			if strings.HasPrefix(strings.TrimSpace(part), "for=") {
				ip := strings.TrimPrefix(strings.TrimSpace(part), "for=")
				ip = strings.Trim(ip, "\"")
				// 移除端口
				if idx := strings.LastIndex(ip, ":"); idx != -1 {
					ip = ip[:idx]
				}
				// 移除IPv6方括号
				ip = strings.Trim(ip, "[]")
				if net.ParseIP(ip) != nil {
					return ip
				}
			}
		}
	}

	// 回退到RemoteAddr
	return remoteIP
}

// realAddr 实现net.Addr接口,用于存储真实IP地址
type realAddr struct {
	network string
	addr    string
}

func (r *realAddr) Network() string {
	return r.network
}

func (r *realAddr) String() string {
	return r.addr
}

// newRealAddr 创建realAddr实例
// 参数: addr - IP地址字符串
// 返回: net.Addr接口实现
func (s *Server) newRealAddr(addr string) net.Addr {
	return &realAddr{
		network: "tcp",
		addr:    addr,
	}
}
