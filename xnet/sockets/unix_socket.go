//go:build !windows

package sockets

import (
	"net"
	"os"
	"syscall"
)

// SockOption Unix Socket 文件属性配置选项的函数类型，使用函数选项模式。
type SockOption func(string) error

// WithChown 返回设置 Unix Socket 文件的所有者（uid/gid）的选项函数。
//
// 通过设置文件归属组，限制只有特定用户组才能连接到该 socket，
// 是 Unix 进程间通信的基本权限控制手段。
func WithChown(uid, gid int) SockOption {
	return func(path string) error {
		if err := os.Chown(path, uid, gid); err != nil {
			return err
		}
		return nil
	}
}

// WithChmod 返回设置 Unix Socket 文件权限模式的选项函数。
//
// 权限模式遵循 Unix 标准（如 0660 表示所有者和组可读写，其他人无访问权限），
// 结合 WithChown 可精确控制 socket 的访问权限。
func WithChmod(mask os.FileMode) SockOption {
	return func(path string) error {
		if err := os.Chmod(path, mask); err != nil {
			return err
		}
		return nil
	}
}

// NewUnixSocketWithOpts 创建具有自定义权限的 Unix Domain Socket 监听器。
//
// 权限竞态问题（TOCTOU）及解决方案：
// net.Listen 创建 socket 后，权限设置之前存在短暂时间窗口，
// 期间 socket 的实际权限由 (0666 & ~umask) 决定，可能过于宽松。
//
// 解决方案：在调用 net.Listen 前临时将 umask 设为 0777，
// 迫使 socket 以 000 权限（任何人都无法访问）创建，
// 然后立即恢复 umask 并通过 WithChmod 设置期望的权限，
// 将权限宽松窗口从 socket 创建时刻压缩到接近零。
//
// 注意：修改 umask 影响整个进程，在多线程环境下可能影响同期创建的其他文件，
// 但因时间极短（微秒级）且通常在启动阶段执行，实际风险可接受。
func NewUnixSocketWithOpts(path string, opts ...SockOption) (net.Listener, error) {
	// 删除已存在的旧 socket 文件，防止 bind 失败（操作系统不允许重复绑定路径）
	if err := syscall.Unlink(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// 临时设置 umask 为 0777，使 socket 以 000 权限创建（防止权限宽松窗口）
	origUmask := syscall.Umask(0o777)
	l, err := net.Listen("unix", path)
	syscall.Umask(origUmask) // 尽快恢复 umask，减少对其他并发文件操作的影响
	if err != nil {
		return nil, err
	}

	// 逐一应用配置选项（如 WithChown/WithChmod）
	for _, op := range opts {
		if err = op(path); err != nil {
			_ = l.Close()
			return nil, err
		}
	}

	return l, nil
}

// NewUnixSocket 创建 Unix Socket 监听器，使用 root:gid 所有权和 0660 权限。
//
// 0660 权限（所有者和组可读写，其他人无权限）适合服务端进程使用 root 运行
// 但客户端进程使用特定 gid 的场景，避免 socket 文件对所有用户开放。
func NewUnixSocket(path string, gid int) (net.Listener, error) {
	return NewUnixSocketWithOpts(path, WithChown(0, gid), WithChmod(0o660))
}
