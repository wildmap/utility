package xnet

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// upgrader configures the WebSocket upgrader with:
// - 10-second handshake timeout
// - 4KB read/write buffers
// - Permissive origin checking (allows all origins)
var upgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   4096,
	WriteBufferSize:  4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin
	},
}

// startWebsocketServer runs the WebSocket server.
// This method runs in a separate goroutine and serves WebSocket connections
// through an HTTP server.
func (s *TCPServer) startWebsocketServer() {
	defer s.wg.Done()

	// Configure HTTP server for WebSocket connections
	httpSvr := &http.Server{
		Handler:           http.HandlerFunc(s.handleWebsocketConn),
		ReadHeaderTimeout: 120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    MaxMsgLen,
		BaseContext: func(_ net.Listener) context.Context {
			return s.ctx
		},
	}

	// Start serving (blocking)
	if err := httpSvr.Serve(s.wsLn); err != nil {
		slog.Error(fmt.Sprintf("Websocket server error: %v", err))
	}
}

// handleWebsocketConn handles HTTP requests and upgrades them to WebSocket connections.
// It performs the following steps:
// 1. Checks if server is shutting down
// 2. Upgrades HTTP connection to WebSocket
// 3. Wraps connection and delegates to common handler
//
// Recovers from panics to prevent the server from crashing.
func (s *TCPServer) handleWebsocketConn(w http.ResponseWriter, r *http.Request) {
	defer func() {
		// Recover from panic
		if rr := recover(); rr != nil {
			slog.Error(fmt.Sprintf("ws handler panic, error: %v", rr))
		}
	}()

	// Check if server is shutting down
	select {
	case <-s.ctx.Done():
		http.Error(w, "Server is shutting down", http.StatusServiceUnavailable)
		return
	default:
	}

	// Upgrade HTTP connection to WebSocket protocol
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade already sent an error response to the client
		return
	}
	// Extract real client IP
	remoteAddr := s.newRealAddr(s.getHTTPClientIP(r))

	// Handle the WebSocket connection
	s.handleConn(NewWSConn(conn, remoteAddr))
}

// getHTTPClientIP 从HTTP请求中获取客户端真实IP
// 优先级：
// 1. 如果RemoteAddr是可信代理，则从X-Forwarded-For获取第一个非内网IP
// 2. 如果RemoteAddr是可信代理，则从X-Real-IP获取
// 3. 直接使用RemoteAddr
func (s *TCPServer) getHTTPClientIP(r *http.Request) string {
	// 获取RemoteAddr的IP部分
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
	}

	// 从X-Forwarded-For获取（优先）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For可能包含多个IP，格式: client, proxy1, proxy2
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// 从X-Real-IP获取（备选）
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// 从Forwarded标准头获取（RFC 7239）
	if forwarded := r.Header.Get("Forwarded"); forwarded != "" {
		// Forwarded: for=192.0.2.60;proto=http;by=203.0.113.43
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

// realAddr 实现net.Addr接口，用于存储真实IP
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
func (s *TCPServer) newRealAddr(addr string) net.Addr {
	return &realAddr{
		network: "tcp",
		addr:    addr,
	}
}
