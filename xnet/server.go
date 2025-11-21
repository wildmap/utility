package xnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
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

// MaxMsgLen defines the maximum allowed message size (50MB)
const MaxMsgLen = 50 * 1024 * 1024

// Server implements a TCP&UDP server that supports both regular socket and WebSocket connections.
// It uses connection multiplexing (cmux) to route connections based on protocol.
// Features:
// - Concurrent connection handling
// - Graceful shutdown
// - Connection tracking and limiting
// - Custom agent factory for connection handling
type Server struct {
	wg        sync.WaitGroup          // WaitGroup for tracking active goroutines
	ctx       context.Context         // Context for shutdown signaling
	cancel    context.CancelFunc      // Cancel function for context
	addr      string                  // Listen address
	newAgent  func(conn IConn) IAgent // Factory function for creating agents
	lns       []net.Listener          // listeners
	conns     []IConn                 // Set of active connections
	connCount atomic.Int32            // Atomic counter for active connections
	started   atomic.Bool             // server started
}

// NewServer creates a new TCP server instance.
func NewServer(addr string, newAgent func(conn IConn) IAgent) *Server {
	return &Server{
		addr:     addr,
		newAgent: newAgent,
		conns:    []IConn{},
	}
}

// validate checks if the server configuration is valid.
func (s *Server) validate() error {
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
func (s *Server) Start() error {
	// Validate configuration
	err := s.validate()
	if err != nil {
		return fmt.Errorf("validate failed: %w", err)
	}

	// Initialize context for shutdown control
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// start tcp listener
	err = s.listenTcpOrPipe(s.addr)
	if err != nil {
		xlog.Errorf("server failed to start: %v", err)
		return err
	}
	// start kcp listener
	err = s.listenKcp(s.addr)
	if err != nil {
		xlog.Errorf("server failed to start: %v", err)
		return err
	}
	// start pipe listener
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
	// Create main listener
	ln, err := s.newTcpListener(s.ctx, addr)
	if err != nil {
		return fmt.Errorf("create tcp listener failed: %w", err)
	}

	s.lns = append(s.lns, ln)
	// Set up connection multiplexer
	cmuxSvr := cmux.New(ln)
	cmuxSvr.SetReadTimeout(5 * time.Second)

	// Match HTTP/WebSocket connections
	wsLn := cmuxSvr.Match(cmux.HTTP1Fast())
	s.lns = append(s.lns, wsLn)

	// Match all other connections
	socketLn := cmuxSvr.Match(cmux.Any())
	s.lns = append(s.lns, socketLn)

	// Start Websocket and socket handlers
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
	// Then wrap with ProxyProtocol for HAProxy PROXY protocol support
	ln = &proxyproto.Listener{Listener: ln}
	return ln, nil
}

// Stop gracefully shuts down the server.
// The shutdown sequence:
// 1. Cancels context to stop accepting new connections
// 2. Closes the main listener
// 3. Closes all active connections
// 4. Waits for all goroutines to finish
func (s *Server) Stop() {
	if !s.started.CompareAndSwap(true, false) {
		xlog.Warnf("server already stopped")
		return
	}

	xlog.Infof("shutting down server...")

	// Cancel context to stop accepting new connections
	if s.cancel != nil {
		s.cancel()
	}

	for _, ln := range s.lns {
		_ = ln.Close()
	}

	// Close all active connections
	for _, conn := range s.conns {
		_ = conn.Close()
	}

	// Wait for all goroutines to finish
	s.wg.Wait()

	xlog.Infof("server stopped")
}

// GetConnCount returns the current number of active connections.
func (s *Server) GetConnCount() int32 {
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
func (s *Server) handleConn(conn IConn) {
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
		xlog.Errorf("%s agent OnInit error: %v", conn.RemoteAddr(), err)
		return
	}

	// Register connection
	s.conns = append(s.conns, conn)

	// Increment connection counter
	s.connCount.Add(1)

	// Start agent processing in a goroutine
	s.wg.Go(func() {
		defer func() {
			// Recover from panic
			if rr := recover(); rr != nil {
				xlog.Errorf("%s agent panic, error: %v\n %s", conn.RemoteAddr(), rr, string(debug.Stack()))
			}

			// Cleanup agent
			agent.OnClose(s.ctx)

			// Close connection
			_ = conn.Close()

			// Unregister connection
			s.conns = slices.DeleteFunc(s.conns, func(c IConn) bool {
				return c == conn
			})

			// Decrement connection counter
			s.connCount.Add(-1)
		}()

		// Run agent's main processing loop
		agent.Run(s.ctx)
	})
}
