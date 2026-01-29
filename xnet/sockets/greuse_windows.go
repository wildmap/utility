//go:build windows

package sockets

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// See net.RawConn.Control
func control(network, address string, c syscall.RawConn) (err error) {
	e := c.Control(func(fd uintptr) {
		if err = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1); err != nil {
			return
		}
	})
	if e != nil {
		return e
	}
	return
}
