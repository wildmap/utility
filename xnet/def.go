package xnet

import (
	"net"
)

// IConn defines the interface for network connections.
// It provides methods for reading/writing messages and accessing connection addresses.
type IConn interface {
	// ReadMsg reads a complete message from the connection.
	// Returns the message data or an error if reading fails.
	ReadMsg() ([]byte, error)

	// WriteMsg writes a complete message to the connection.
	// Returns an error if writing fails.
	WriteMsg(args []byte) error

	// LocalAddr returns the local network address.
	LocalAddr() net.Addr

	// RemoteAddr returns the remote network address.
	RemoteAddr() net.Addr

	// Close closes the connection.
	// Any blocked Read or Write operations will be unblocked and return errors.
	Close() error
}

// IAgent defines the interface for network agents that handle client connections.
// Implementations must provide initialization, main processing loop, and cleanup logic.
type IAgent interface {
	// OnInit performs initialization tasks when a connection is established.
	// Returns an error if initialization fails.
	OnInit() error

	// Run executes the main processing loop for handling client requests.
	// This method blocks until the connection is closed or an error occurs.
	Run()

	// OnClose performs cleanup tasks when the connection is being closed.
	OnClose()
}
