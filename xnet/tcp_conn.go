package xnet

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

const (
	// MsgLenSize is the size in bytes of the message length field (4 bytes for uint32)
	MsgLenSize = 4
)

// TCPConn wraps a net.Conn to provide length-prefixed message reading/writing.
// Message format: [4-byte length][message body]
// Thread-safe for concurrent writes via mutex.
type TCPConn struct {
	conn       net.Conn   // Underlying network connection
	writeMutex sync.Mutex // Mutex to ensure thread-safe writes
}

// NewTCPConn creates a new TCPConn wrapping the given net.Conn.
func NewTCPConn(conn net.Conn) *TCPConn {
	return &TCPConn{
		conn: conn,
	}
}

// Close closes the underlying connection.
func (t *TCPConn) Close() error {
	return t.conn.Close()
}

// RemoteAddr returns the remote network address.
func (t *TCPConn) RemoteAddr() net.Addr {
	return t.conn.RemoteAddr()
}

// LocalAddr returns the local network address.
func (t *TCPConn) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

// ReadMsg reads a length-prefixed message from the TCP connection.
// Message format: [4-byte big-endian length][message body]
func (t *TCPConn) ReadMsg() ([]byte, error) {
	// Step 1: Read the 4-byte message length header
	var msgLenBuf [MsgLenSize]byte
	if _, err := io.ReadFull(t.conn, msgLenBuf[:]); err != nil {
		return nil, fmt.Errorf("read msg length failed: %w", err)
	}

	// Step 2: Parse the message length (big-endian)
	msgLen := binary.BigEndian.Uint32(msgLenBuf[:])

	// Step 3: Validate message length
	if msgLen == 0 || msgLen > MaxMsgLen {
		return nil, errors.New("read msg length out of range")
	}

	// Step 4: Read the message body
	msgData := make([]byte, msgLen)
	if _, err := io.ReadFull(t.conn, msgData[:msgLen]); err != nil {
		return nil, fmt.Errorf("read msg body failed: %w", err)
	}

	return msgData, nil
}

// WriteMsg writes a length-prefixed message to the TCP connection.
// Message format: [4-byte big-endian length][message body]
// This method is thread-safe and can be called concurrently.
func (t *TCPConn) WriteMsg(data []byte) error {
	t.writeMutex.Lock()
	defer t.writeMutex.Unlock()

	msgLen := uint32(len(data))

	// Skip if message is empty
	if msgLen == 0 {
		return nil
	}

	// Construct complete message: [4-byte length][message body]
	msg := make([]byte, MsgLenSize+msgLen)
	binary.BigEndian.PutUint32(msg[:MsgLenSize], msgLen)
	copy(msg[MsgLenSize:], data)

	// Write to TCP connection
	if _, err := t.conn.Write(msg); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}
