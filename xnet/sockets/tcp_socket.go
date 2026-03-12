// sockets 包提供创建和配置 Unix Domain Socket 及 TCP Socket 的辅助函数。
package sockets

import (
	"context"
	"crypto/tls"
	"net"
)

// NewTCPSocket 创建 TCP 监听器，可选支持 TLS 加密。
//
// 关键配置：
//   - net.ListenConfig.Control：注入 SO_REUSEADDR 和 SO_REUSEPORT socket 选项
//     允许多个进程监听同一端口（用于零停机重启），以及快速重用 TIME_WAIT 状态的端口
//   - SetMultipathTCP(true)：启用 MPTCP（多路径 TCP），允许单个连接同时使用多个网络路径
//     可提升高可用场景下的带宽利用率（操作系统不支持时自动降级为普通 TCP）
//   - TLS 支持：当 tlsConfig 不为 nil 时，用 tls.NewListener 包装实现透明 TLS 升级
//
// NextProtos 设置为 ["http/1.1"] 告知 TLS 客户端本服务器支持的应用层协议（ALPN）。
func NewTCPSocket(ctx context.Context, addr string, tlsConfig *tls.Config) (net.Listener, error) {
	lc := &net.ListenConfig{
		Control: control, // 平台特定的 socket 选项设置（SO_REUSEADDR/SO_REUSEPORT）
	}
	lc.SetMultipathTCP(true) // 启用 MPTCP，操作系统不支持时静默降级
	l, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		tlsConfig.NextProtos = []string{"http/1.1"} // ALPN 协议协商
		l = tls.NewListener(l, tlsConfig)
	}
	return l, nil
}
