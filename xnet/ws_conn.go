package xnet

import (
	"net"
	"sync"

	"github.com/gorilla/websocket"
)

// WSConn wraps a gorilla/websocket connection to implement the IConn interface.
// It provides thread-safe message reading/writing for WebSocket connections.
type WSConn struct {
	conn       *websocket.Conn // Underlying WebSocket connection
	remoteAddr net.Addr        // Real remote address (from proxy headers or direct connection)
	writeMutex sync.Mutex      // Mutex to ensure thread-safe writes
}

// NewWSConn creates a new WSConn wrapping the given WebSocket connection.
func NewWSConn(conn *websocket.Conn, remoteAddr net.Addr) *WSConn {
	if remoteAddr == nil {
		remoteAddr = conn.RemoteAddr()
	}
	return &WSConn{
		conn:       conn,
		remoteAddr: remoteAddr,
	}
}

// Close closes the WebSocket connection.
func (c *WSConn) Close() error {
	return c.conn.Close()
}

// RemoteAddr returns the remote network address.
func (c *WSConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// LocalAddr returns the local network address.
func (c *WSConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// ReadMsg reads a message from the WebSocket connection.
func (c *WSConn) ReadMsg() ([]byte, error) {
	_, data, err := c.conn.ReadMessage()
	return data, err
}

// WriteMsg writes a binary message to the WebSocket connection in a thread-safe manner.
func (c *WSConn) WriteMsg(data []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	err := c.conn.WriteMessage(websocket.TextMessage, data)
	return err
}
