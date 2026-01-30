//go:build !windows && !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package sockets

import (
	"syscall"
)

// See net.RawConn.Control
func control(network, address string, c syscall.RawConn) (err error) {
	return nil
}
