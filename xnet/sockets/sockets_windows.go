//go:build windows

package sockets

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Microsoft/go-winio"
)

// configureUnixTransport 在 Windows 平台上返回协议不可用错误。
//
// Windows 从 Build 17063 开始支持 Unix Domain Socket，但当前实现不使用它，
// 统一使用 Named Pipe（命名管道）作为 Windows 上的本地进程间通信方案。
func configureUnixTransport(tr *http.Transport, proto, addr string) error {
	return ErrProtocolNotAvailable
}

// configureNpipeTransport 为 Windows 命名管道协议配置 HTTP Transport。
//
// 禁用 HTTP 压缩：命名管道为本地通信，无网络延迟，压缩无收益。
// 通过自定义 DialContext 使用 go-winio 库建立命名管道连接，
// 绕过标准 net.Dialer（标准库不直接支持命名管道协议）。
func configureNpipeTransport(tr *http.Transport, proto, addr string) error {
	tr.DisableCompression = true
	tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return winio.DialPipeContext(ctx, addr)
	}
	return nil
}

// DialPipe 连接到 Windows 命名管道。
func DialPipe(addr string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(addr, &timeout)
}

// PipePath 返回 Windows 命名管道的路径。
//
// Windows 命名管道路径格式：`\\.\pipe\{name}`，
// 表示本机（.）上的命名管道（pipe）目录中名为 {name} 的管道。
func PipePath(name string) string {
	return fmt.Sprintf(`//./pipe/%s`, name)
}

// ListenPipePath 返回 Windows 平台上管道监听地址，格式为 "npipe://{路径}"。
//
// 此格式与 server.go 中的协议解析逻辑（strings.Cut(addr, "://")）兼容，
// 解析后协议为 "npipe"，由 listeners.New 路由到命名管道处理逻辑。
func ListenPipePath(name string) string {
	return fmt.Sprintf(`npipe://%s`, PipePath(name))
}
