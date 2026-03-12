//go:build !windows && !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package sockets

import (
	"syscall"
)

// control 在不支持 SO_REUSEPORT 的其他平台上为空操作，不设置任何 socket 选项。
//
// 此版本覆盖了所有不在主流 Unix/Windows 列表中的操作系统（如 Plan 9 等），
// 提供编译兼容性而不影响功能，net.ListenConfig.Control 需要此函数签名。
func control(network, address string, c syscall.RawConn) (err error) {
	return nil
}
