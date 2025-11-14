package xnet

import (
	"context"
	"net"

	"github.com/pires/go-proxyproto"
)

// NewListener creates a new TCP listener with proxy protocol support.
// The listener is wrapped with:
// 1. Keepalive wrapper for TCP keepalive functionality
// 2. ProxyProtocol wrapper for HAProxy PROXY protocol support
func NewListener(ctx context.Context, addr string) (net.Listener, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
	}

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	// Wrap with Keepalive for TCP keepalive functionality
	// Then wrap with ProxyProtocol for HAProxy PROXY protocol support
	ln = &proxyproto.Listener{Listener: &Keepalive{ln}}
	return ln, nil
}
