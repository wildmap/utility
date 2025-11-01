package xnet

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// Keepalive wraps a net.Listener to add TCP keepalive functionality.
// It configures keepalive settings for all accepted connections.
type Keepalive struct {
	Listener net.Listener
}

// Accept waits for and returns the next connection to the listener.
// Each accepted connection is configured with TCP keepalive settings:
// - Keepalive is enabled
// - Keepalive period is set to 10 seconds
// - Read deadline is set to 120 seconds
//
// Detection time calculation:
// - Linux default: 30s + 8 * 30s = 270s
// - macOS default: 30s + 8 * 75s = 630s
func (kln *Keepalive) Accept() (net.Conn, error) {
	c, err := kln.Listener.Accept()
	if err != nil {
		return nil, err
	}

	kac, err := kln.createKeepaliveConn(c)
	if err != nil {
		return nil, fmt.Errorf("create keepalive connection failed, %w", err)
	}

	// Enable TCP keepalive
	if err = kac.SetKeepAlive(true); err != nil {
		return nil, fmt.Errorf("SetKeepAlive failed, %w", err)
	}

	// Set keepalive period to 10 seconds
	if err = kac.SetKeepAlivePeriod(10 * time.Second); err != nil {
		return nil, fmt.Errorf("SetKeepAlivePeriod failed, %w", err)
	}

	// Set initial read deadline to 120 seconds
	if err = kac.SetReadDeadline(time.Now().Add(120 * time.Second)); err != nil {
		return nil, fmt.Errorf("SetReadDeadline failed, %w", err)
	}

	return kac, nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (kln *Keepalive) Close() error {
	if kln.Listener == nil {
		return nil
	}
	return kln.Listener.Close()
}

// Addr returns the listener's network address.
func (kln *Keepalive) Addr() net.Addr {
	if kln.Listener == nil {
		return nil
	}
	return kln.Listener.Addr()
}

// createKeepaliveConn creates a keepAliveConn from a net.Conn.
// Returns an error if the connection is not a TCP connection.
func (kln *Keepalive) createKeepaliveConn(c net.Conn) (*keepAliveConn, error) {
	tcpc, ok := c.(*net.TCPConn)
	if !ok {
		return nil, errors.New("only tcp connections have keepalive")
	}
	return &keepAliveConn{tcpc}, nil
}

// keepAliveConn wraps a TCP connection to provide keepalive functionality.
type keepAliveConn struct {
	*net.TCPConn
}

// SetKeepAlive sets whether the operating system should send keepalive messages.
func (l *keepAliveConn) SetKeepAlive(doKeepAlive bool) error {
	return l.TCPConn.SetKeepAlive(doKeepAlive)
}

// SetKeepAlivePeriod sets the period between TCP keepalive probes.
func (l *keepAliveConn) SetKeepAlivePeriod(d time.Duration) error {
	return l.TCPConn.SetKeepAlivePeriod(d)
}
