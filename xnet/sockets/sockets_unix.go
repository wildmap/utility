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

// maxUnixSocketPathSize Unix Domain Socket 路径的最大字节长度，受操作系统限制。
//
// 由 syscall.RawSockaddrUnix.Path 的长度决定（通常为 108 字节），
// 路径过长会导致 bind 系统调用失败。
const maxUnixSocketPathSize = len(syscall.RawSockaddrUnix{}.Path)

// configureUnixTransport 为 Unix Domain Socket 协议配置 HTTP Transport。
//
// 禁用 HTTP 压缩：Unix Socket 为本地通信，无网络延迟，
// 压缩/解压缩只会增加 CPU 开销而无带宽收益。
// 通过自定义 DialContext 将所有 HTTP 请求路由到 Unix Socket 文件路径，
// 忽略 HTTP 请求中的 host 和端口信息（本地套接字不需要）。
func configureUnixTransport(tr *http.Transport, proto, addr string) error {
	if len(addr) > maxUnixSocketPathSize {
		return fmt.Errorf("unix socket path %q is too long", addr)
	}
	tr.DisableCompression = true // 本地通信无需压缩
	dialer := &net.Dialer{
		Timeout: defaultTimeout,
	}
	tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, proto, addr)
	}
	return nil
}

// configureNpipeTransport 在非 Windows 平台上返回协议不可用错误。
func configureNpipeTransport(tr *http.Transport, proto, addr string) error {
	return ErrProtocolNotAvailable
}

// DialPipe 在非 Windows 平台上模拟命名管道连接，实际使用 Unix Socket。
//
// 提供跨平台接口统一：Windows 上连接命名管道，Unix 上连接 Unix Socket，
// 上层代码无需感知平台差异。
func DialPipe(addr string, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	return dialer.Dial("unix", addr)
}

// PipePath 返回 Unix 平台上管道文件的路径。
//
// 遵循 Linux 惯例：进程间通信 socket 文件放在 /var/run/ 目录下，
// 使用 .sock 后缀区分于普通文件。
func PipePath(name string) string {
	return fmt.Sprintf("/var/run/%s.sock", name)
}

// ListenPipePath 返回 Unix 平台上管道监听地址，格式为 "unix://{路径}"。
//
// 此格式与 server.go 中的协议解析逻辑（strings.Cut(addr, "://")）兼容。
func ListenPipePath(name string) string {
	return fmt.Sprintf(`unix://%s`, PipePath(name))
}
