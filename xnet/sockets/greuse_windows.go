//go:build windows

package sockets

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// control 在 Windows 上设置 SO_REUSEADDR socket 选项。
//
// Windows 不支持 Linux 意义上的 SO_REUSEPORT（多进程绑定同一端口），
// 仅设置 SO_REUSEADDR 以允许快速重用 TIME_WAIT 状态的端口，
// 防止服务重启时因端口占用而无法立即监听。
//
// 注意：Windows 的 SO_REUSEADDR 语义与 Linux 不同，
// 在 Windows 上它还允许多个进程绑定同一端口（可能引发安全问题），
// 实际使用中需结合 ACL 控制访问权限。
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
