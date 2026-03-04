package xnet

import (
	"net"
	"runtime/debug"
	"time"

	"github.com/wildmap/utility/xlog"
)

// startSocketServer 运行 TCP/KCP Socket 连接的接受循环，阻塞直到监听器关闭。
//
// 对临时网络错误（如 EAGAIN、ECONNABORTED）采用指数退避策略，
// 从 5ms 开始，每次翻倍，上限 1 秒，防止错误风暴下的 CPU 空转。
// 服务器关闭时通过 ctx.Done() 信号提前退出，避免记录不必要的关闭错误日志。
func (s *Server) startSocketServer(ln net.Listener) {
	var tempDelay time.Duration
	const maxDelay = 1 * time.Second

	for {
		// 优先检查服务器是否正在关闭，避免在关闭期间记录大量错误日志
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
			}

			// 对临时网络错误（如 accept 队列满）使用指数退避，避免 CPU 空转
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

			xlog.Errorf("accept failed, error %v", err)
			return
		}

		tempDelay = 0 // 成功接受连接后重置退避时间

		s.handleSocketConn(conn)
	}
}

// handleSocketConn 封装原始 net.Conn 为 SocketConn 后委托给统一处理逻辑。
//
// 通过 recover 捕获连接初始化阶段的 panic（如 newAgent 函数中的异常），
// 防止单个连接的异常导致整个接受循环崩溃。
func (s *Server) handleSocketConn(conn net.Conn) {
	defer func() {
		if rr := recover(); rr != nil {
			xlog.Errorf("socket handler panic, error: %v\n%s", rr, string(debug.Stack()))
		}
	}()

	s.handleConn(NewSocketConn(conn))
}
