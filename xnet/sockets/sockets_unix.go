//go:build !windows

package sockets

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

const maxUnixSocketPathSize = len(syscall.RawSockaddrUnix{}.Path)

func configureUnixTransport(tr *http.Transport, proto, addr string) error {
	if len(addr) > maxUnixSocketPathSize {
		return fmt.Errorf("unix socket path %q is too long", addr)
	}
	// No need for compression in local communications.
	tr.DisableCompression = true
	dialer := &net.Dialer{
		Timeout: defaultTimeout,
	}
	tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, proto, addr)
	}
	return nil
}

func configureNpipeTransport(tr *http.Transport, proto, addr string) error {
	return ErrProtocolNotAvailable
}

// DialPipe connects to a Windows named pipe.
// This is not supported on other OSes.
func DialPipe(addr string, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	return dialer.Dial("unix", addr)
}

func PipePath(name string) string {
	return fmt.Sprintf("/var/run/%s.sock", name)
}

func ListenPipePath(name string) string {
	return fmt.Sprintf(`unix://%s`, PipePath(name))
}
