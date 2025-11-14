package xnet

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/soheilhy/cmux"
)

// MaxMsgLen defines the maximum allowed message size (50MB)
const MaxMsgLen = 50 * 1024 * 1024

// ConnSet is a set of active connections
type ConnSet map[IConn]struct{}

// TCPServer implements a TCP server that supports both regular socket and WebSocket connections.
// It uses connection multiplexing (cmux) to route connections based on protocol.
// Features:
// - Concurrent connection handling
// - Graceful shutdown
// - Connection tracking and limiting
// - Custom agent factory for connection handling
type TCPServer struct {
	wg         sync.WaitGroup          // WaitGroup for tracking active goroutines
	ctx        context.Context         // Context for shutdown signaling
	cancel     context.CancelFunc      // Cancel function for context
	addr       string                  // Listen address
	newAgent   func(conn IConn) IAgent // Factory function for creating agents
	lns        []net.Listener          // listeners
	cmux       cmux.CMux               // Connection multiplexer
	conns      ConnSet                 // Set of active connections
	mutexConns sync.RWMutex            // Read-write mutex for connection set
	connCount  atomic.Int32            // Atomic counter for active connections
	started    atomic.Bool             // server started
}

// NewTCPServer creates a new TCP server instance.
func NewTCPServer(addr string, newAgent func(conn IConn) IAgent) *TCPServer {
	return &TCPServer{
		addr:     addr,
		newAgent: newAgent,
	}
}

// validate checks if the server configuration is valid.
func (s *TCPServer) validate() error {
	if s.addr == "" {
		return errors.New("addr is empty")
	}

	if s.newAgent == nil {
		return errors.New("NewAgent must not be nil")
	}

	return nil
}

// Start initializes and starts the TCP server.
// This method blocks until the server is stopped or encounters an error.
//
// The startup sequence:
// 1. Validates server configuration
// 2. Initializes context and connection tracking
// 3. Creates main listener
// 4. Sets up connection multiplexer for socket and WebSocket
// 5. Starts socket and WebSocket handlers in separate goroutines
// 6. Begins accepting connections (blocking)
func (s *TCPServer) Start() error {
	// Validate configuration
	err := s.validate()
	if err != nil {
		return fmt.Errorf("validate failed: %w", err)
	}

	// Initialize context for shutdown control
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Initialize connection tracking
	s.conns = make(ConnSet)

	// Create main listener
	ln, err := NewListener(s.ctx, s.addr)
	if err != nil {
		return fmt.Errorf("create listener failed: %w", err)
	}

	if !s.started.CompareAndSwap(false, true) {
		return errors.New("server already started")
	}

	s.lns = append(s.lns, ln)
	// Set up connection multiplexer
	s.cmux = cmux.New(ln)
	s.cmux.SetReadTimeout(30 * time.Second)

	// Match HTTP/WebSocket connections
	wsLn := s.cmux.Match(cmux.HTTP1Fast())
	s.lns = append(s.lns, wsLn)

	// Match all other connections
	socketLn := s.cmux.Match(cmux.Any())
	s.lns = append(s.lns, socketLn)

	// Start WebSocket and socket handlers
	s.wg.Go(func() {
		s.startWebsocketServer(wsLn)
	})
	s.wg.Go(func() {
		s.startSocketServer(socketLn)
	})
	s.wg.Go(func() {
		_ = s.cmux.Serve()
	})

	slog.Info(fmt.Sprintf("tcp server listening at %s", s.addr))
	return nil
}

// Stop gracefully shuts down the server.
// The shutdown sequence:
// 1. Cancels context to stop accepting new connections
// 2. Closes the main listener
// 3. Closes all active connections
// 4. Waits for all goroutines to finish
func (s *TCPServer) Stop() error {
	slog.Info(fmt.Sprintf("shutting down tcp server..."))

	// Cancel context to stop accepting new connections
	if s.cancel != nil {
		s.cancel()
	}

	for _, ln := range s.lns {
		_ = ln.Close()
	}

	if s.cmux != nil {
		s.cmux.Close()
	}

	// Close all active connections
	s.mutexConns.Lock()
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.conns = make(ConnSet)
	s.mutexConns.Unlock()

	// Wait for all goroutines to finish
	s.wg.Wait()

	slog.Info(fmt.Sprintf("tcp server stopped"))
	return nil
}

// GetConnCount returns the current number of active connections.
func (s *TCPServer) GetConnCount() int32 {
	return s.connCount.Load()
}

// handleConn handles a new connection by:
// 1. Creating an agent using the factory function
// 2. Initializing the agent
// 3. Registering the connection
// 4. Starting the agent's processing loop in a goroutine
// 5. Cleaning up when the agent finishes
//
// This method is called for both socket and WebSocket connections.
func (s *TCPServer) handleConn(conn IConn) {
	// Create agent for this connection
	agent := s.newAgent(conn)
	if agent == nil {
		_ = conn.Close()
		return
	}

	// Initialize agent
	if err := agent.OnInit(s.ctx); err != nil {
		agent.OnClose(s.ctx)
		_ = conn.Close()
		slog.Error(fmt.Sprintf("%s agent OnInit error: %v", conn.RemoteAddr(), err))
		return
	}

	// Register connection
	s.mutexConns.Lock()
	s.conns[conn] = struct{}{}
	s.mutexConns.Unlock()

	// Increment connection counter
	s.connCount.Add(1)

	// Start agent processing in a goroutine
	s.wg.Go(func() {
		defer func() {
			// Recover from panic
			if rr := recover(); rr != nil {
				slog.Error(fmt.Sprintf("%s agent panic, error: %v\n %s", conn.RemoteAddr(), rr, string(debug.Stack())))
			}

			// Cleanup agent
			agent.OnClose(s.ctx)

			// Close connection
			_ = conn.Close()

			// Unregister connection
			s.mutexConns.Lock()
			delete(s.conns, conn)
			s.mutexConns.Unlock()

			// Decrement connection counter
			s.connCount.Add(-1)
		}()

		// Run agent's main processing loop
		agent.Run(s.ctx)
	})
}
