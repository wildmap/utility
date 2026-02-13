package xnet

import (
	"net"
	"runtime/debug"
	"time"

	"github.com/wildmap/utility/xlog"
)

// startSocketServer 运行socket连接接受循环
// 该方法在独立的goroutine中运行,接受常规TCP socket连接
// 对临时接受错误实现指数退避策略
func (s *Server) startSocketServer(ln net.Listener) {
	var tempDelay time.Duration
	const maxDelay = 1 * time.Second

	for {
		// 检查服务器是否正在关闭
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 接受新连接
		conn, err := ln.Accept()
		if err != nil {
			// 再次检查是否因关闭导致错误
			select {
			case <-s.ctx.Done():
				return
			default:
			}

			// 对临时网络错误使用指数退避策略
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if tempDelay > maxDelay {
					tempDelay = maxDelay
				}
				xlog.Warnf("accept error, retrying, delay: %d, err: %v", tempDelay, err)
				time.Sleep(tempDelay)
				continue
			}

			// 非临时错误,记录日志并返回
			xlog.Errorf("accept failed, error %v", err)
			return
		}

		// 成功接受连接后重置延迟
		tempDelay = 0

		// 处理新连接
		s.handleSocketConn(conn)
	}
}

// handleSocketConn 封装原始socket连接并委托给通用处理器
// 从panic中恢复以防止服务器崩溃
func (s *Server) handleSocketConn(conn net.Conn) {
	defer func() {
		// 从panic中恢复
		if rr := recover(); rr != nil {
			xlog.Errorf("socket handler panic, error: %v\n%s", rr, string(debug.Stack()))
		}
	}()

	s.handleConn(NewSocketConn(conn))
}
