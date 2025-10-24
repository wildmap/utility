package xnet

import (
	"fmt"
	"log/slog"
	"net"
	"time"
)

// startSocketServer runs the socket connection acceptor loop.
// This method runs in a separate goroutine and accepts regular TCP socket connections.
// It implements exponential backoff for temporary accept errors.
func (s *TCPServer) startSocketServer() {
	defer s.wg.Done()

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
		conn, err := s.socketLn.Accept()
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
				slog.Warn(fmt.Sprintf("accept error, retrying, delay: %d, err: %v", tempDelay, err))
				time.Sleep(tempDelay)
				continue
			}

			// Non-temporary error, log and return
			slog.Error(fmt.Sprintf("accept failed, error %v", err))
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
func (s *TCPServer) handleSocketConn(conn net.Conn) {
	defer func() {
		// Recover from panic
		if rr := recover(); rr != nil {
			slog.Error(fmt.Sprintf("ws handler panic, error: %v", rr))
		}
	}()

	s.handleConn(NewTCPConn(conn))
}
