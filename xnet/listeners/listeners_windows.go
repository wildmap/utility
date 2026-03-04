//go:build windows

package listeners

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"

	"github.com/wildmap/utility/xnet/sockets"
)

// New 根据协议类型创建网络监听器（Windows 平台）。
//
// 支持的协议：
//   - "tcp"：创建带 Keep-Alive 的 TCP 监听器（与非 Windows 平台相同）
//   - "npipe"：创建 Windows 命名管道监听器（本地进程间通信，替代 Unix Socket）
//
// Windows 命名管道配置说明：
//   - SDDL（安全描述符）"D:P(A;;GA;;;BA)(A;;GA;;;SY)"：
//     允许 Administrators（BA）和 SYSTEM（SY）账户具有完全访问权限（GA）
//   - MessageMode=true：启用消息模式，支持 CloseWrite() 操作（区分消息边界）
//   - 64KB 输入/输出缓冲区：平衡内存占用和 I/O 吞吐量
func New(ctx context.Context, proto, addr string, tlsConfig *tls.Config) (net.Listener, error) {
	switch proto {
	case "tcp":
		ln, err := sockets.NewTCPSocket(ctx, addr, tlsConfig)
		if err != nil {
			return nil, err
		}
		return &Keepalive{Listener: ln}, err
	case "npipe":
		// SDDL 安全描述符：D:P 表示保护性 DACL，(A;;GA;;;BA) 允许管理员完全访问，(A;;GA;;;SY) 允许 SYSTEM 完全访问
		sddl := "D:P(A;;GA;;;BA)(A;;GA;;;SY)"
		c := winio.PipeConfig{
			SecurityDescriptor: sddl,
			MessageMode:        true,  // 消息模式确保每次 Read 返回完整消息，而非字节流
			InputBufferSize:    65536, // 64KB 输入缓冲区
			OutputBufferSize:   65536, // 64KB 输出缓冲区
		}
		return winio.ListenPipe(addr, &c)
	default:
		return nil, fmt.Errorf("invalid protocol format: windows only supports tcp and npipe")
	}
}
