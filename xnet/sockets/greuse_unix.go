//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package sockets

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// control 在 Unix 系列操作系统上设置 SO_REUSEADDR 和 SO_REUSEPORT socket 选项。
//
// SO_REUSEADDR：允许绑定处于 TIME_WAIT 状态的端口，
// 防止服务器重启时因端口被占用而无法立即监听。
//
// SO_REUSEPORT：允许多个进程/线程绑定同一端口（内核级负载均衡），
// 适用于多进程并行 Accept 的高性能服务器架构，
// 内核会自动将新连接均衡分配到各监听进程，减少惊群效应（thundering herd）。
//
// 此函数作为 net.ListenConfig.Control 回调传入，在 socket 创建后、bind 之前执行。
func control(network, address string, c syscall.RawConn) error {
	return c.Control(func(fd uintptr) {
		// SO_REUSEADDR：快速重用处于 TIME_WAIT 状态的端口
		if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
			return
		}
		// SO_REUSEPORT：支持多进程监听同一端口（内核负载均衡）
		if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
			return
		}
	})
}
