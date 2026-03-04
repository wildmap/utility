// sockets 包提供创建和配置 Unix Domain Socket 及 TCP Socket 的辅助函数。
package sockets

import (
	"errors"
	"net"
	"net/http"
	"time"
)

// defaultTimeout HTTP 客户端拨号的默认超时时间。
const defaultTimeout = 10 * time.Second

// ErrProtocolNotAvailable 表示当前操作系统不支持指定的传输协议。
//
// 例如：在 Linux 上请求 npipe 协议，或在 Windows 上请求 unix 协议时返回此错误。
var ErrProtocolNotAvailable = errors.New("protocol not available")

// ConfigureTransport 根据协议类型配置 HTTP Transport 的拨号行为。
//
// 设计目的：统一处理不同协议（TCP/Unix Socket/Windows Named Pipe）的连接差异，
// 使上层代码可以透明地通过 HTTP Client 访问不同协议的服务。
//
// 协议行为差异：
//   - unix/npipe：本地通信，禁用压缩（本地通信无网络延迟，压缩只增加 CPU 开销）
//   - 其他协议（tcp）：启用代理和压缩，使用标准网络配置
//
// 注意：若在调用此函数之后还需手动修改压缩设置，须在本函数调用之后进行。
func ConfigureTransport(tr *http.Transport, proto, addr string) error {
	switch proto {
	case "unix":
		return configureUnixTransport(tr, proto, addr)
	case "npipe":
		return configureNpipeTransport(tr, proto, addr)
	default:
		tr.Proxy = http.ProxyFromEnvironment // 从环境变量读取代理配置（HTTP_PROXY 等）
		tr.DisableCompression = false
		tr.DialContext = (&net.Dialer{
			Timeout: defaultTimeout,
		}).DialContext
	}
	return nil
}
