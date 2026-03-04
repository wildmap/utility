//go:build !windows

package listeners

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"

	"github.com/wildmap/utility/xnet/sockets"
)

// New 根据协议类型创建网络监听器（非 Windows 平台）。
//
// 支持的协议：
//   - "tcp"：创建带 Keep-Alive 的 TCP 监听器
//   - "unix"：创建 Unix Domain Socket 监听器（本地进程间通信）
//
// TCP 监听器通过 Keepalive 包装器自动配置 Keep-Alive 参数；
// Unix Socket 使用调用进程的 egid 作为文件所有组，权限为 0660（仅组内可读写）。
func New(ctx context.Context, proto, addr string, tlsConfig *tls.Config) (net.Listener, error) {
	switch proto {
	case "tcp":
		ln, err := sockets.NewTCPSocket(ctx, addr, tlsConfig)
		if err != nil {
			return nil, err
		}
		return &Keepalive{Listener: ln}, err
	case "unix":
		return sockets.NewUnixSocket(addr, os.Getegid())
	default:
		return nil, fmt.Errorf("invalid protocol format: %q", proto)
	}
}
