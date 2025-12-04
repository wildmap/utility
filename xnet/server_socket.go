package xnet

import (
	"net"
	"runtime/debug"
	"time"

	"github.com/wildmap/utility/xlog"
)

// startSocketServer runs the socket connection acceptor loop.
// This method runs in a separate goroutine and accepts regular TCP socket connections.
// It implements exponential backoff for temporary accept errors.
func (s *Server) startSocketServer(ln net.Listener) {
	var tempDelay time.Duration
	const maxDelay = 1 * time.Second

	for {
		// Check if server is shutting down
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Accept new connection
		conn, err := ln.Accept()
		if err != nil {
			// Check again if shutdown caused the error
			select {
			case <-s.ctx.Done():
				return
			default:
			}

			// Handle temporary network errors with exponential backoff
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

			// Non-temporary error, log and return
			xlog.Errorf("accept failed, error %v", err)
			return
		}

		// Reset delay on successful accept
		tempDelay = 0

		// Handle the new connection
		s.handleSocketConn(conn)
	}
}

// handleSocketConn wraps a raw socket connection and delegates to the common handler.
// Recovers from panics to prevent the server from crashing.
func (s *Server) handleSocketConn(conn net.Conn) {
	defer func() {
		// Recover from panic
		if rr := recover(); rr != nil {
			xlog.Errorf("socket handler panic, error: %v\n%s", rr, string(debug.Stack()))
		}
	}()

	s.handleConn(NewSocketConn(conn))
}
