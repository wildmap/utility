package listeners

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// Keepalive 封装 net.Listener，为每个接受的 TCP 连接自动启用 Keep-Alive 机制。
//
// Keep-Alive 的作用：
//   - 周期性发送探测包检测死连接（如客户端进程崩溃、网络中断但 TCP 未收到 RST）
//   - 10 秒探测间隔 + 120 秒读超时，确保长时间无数据的死连接能被及时清理
//   - 防止大量死连接占用服务器文件描述符资源
//
// 探测计时（以 Linux 为例）：
//   - tcp_keepalive_time（首次探测等待时间）+ tcp_keepalive_probes * tcp_keepalive_intvl
//   - Linux 默认：30s + 8 * 30s = 270s；macOS 默认：30s + 8 * 75s = 630s
type Keepalive struct {
	Listener net.Listener
}

// Accept 接受新连接并为其配置 Keep-Alive 参数。
//
// 在底层 Accept 基础上，额外设置：
//   - SetKeepAlive(true)：启用 TCP Keep-Alive 探测
//   - SetKeepAlivePeriod(10s)：将探测间隔设为 10 秒（覆盖操作系统默认值）
//   - SetReadDeadline(120s)：设置读超时，120 秒无数据则连接超时
func (kln *Keepalive) Accept() (net.Conn, error) {
	c, err := kln.Listener.Accept()
	if err != nil {
		return nil, err
	}

	kac, err := kln.createKeepaliveConn(c)
	if err != nil {
		return nil, fmt.Errorf("create keepalive connection failed, %w", err)
	}
	// 启用 TCP Keep-Alive，由操作系统内核发送探测包，不占用应用层线程
	if err = kac.SetKeepAlive(true); err != nil {
		return nil, fmt.Errorf("SetKeepAlive failed, %w", err)
	}
	if err = kac.SetKeepAlivePeriod(10 * time.Second); err != nil {
		return nil, fmt.Errorf("SetKeepAlivePeriod failed, %w", err)
	}
	// 读超时配合 Keep-Alive，确保即使探测未及时触发也能清理长时间无数据的连接
	if err = kac.SetReadDeadline(time.Now().Add(120 * time.Second)); err != nil {
		return nil, fmt.Errorf("SetReadDeadline failed, %w", err)
	}
	return kac, nil
}

// Close 关闭底层监听器。
func (kln *Keepalive) Close() error {
	if kln.Listener == nil {
		return nil
	}
	return kln.Listener.Close()
}

// Addr 返回监听器的网络地址。
func (kln *Keepalive) Addr() net.Addr {
	if kln.Listener == nil {
		return nil
	}
	return kln.Listener.Addr()
}

// createKeepaliveConn 将 net.Conn 转换为 *net.TCPConn 并封装为 keepAliveConn。
//
// Keep-Alive 是 TCP 特有的机制，只有 *net.TCPConn 支持相关设置，
// 其他类型的连接（如 TLS、Unix socket）无法设置 Keep-Alive。
func (kln *Keepalive) createKeepaliveConn(c net.Conn) (*keepAliveConn, error) {
	tcpc, ok := c.(*net.TCPConn)
	if !ok {
		return nil, errors.New("only tcp connections have keepalive")
	}
	return &keepAliveConn{tcpc}, nil
}

// keepAliveConn 封装 *net.TCPConn，提供 Keep-Alive 参数配置能力。
type keepAliveConn struct {
	*net.TCPConn
}

// SetKeepAlive 启用或禁用 TCP Keep-Alive 探测机制。
func (l *keepAliveConn) SetKeepAlive(doKeepAlive bool) error {
	return l.TCPConn.SetKeepAlive(doKeepAlive)
}

// SetKeepAlivePeriod 设置 Keep-Alive 探测包的发送间隔。
func (l *keepAliveConn) SetKeepAlivePeriod(d time.Duration) error {
	return l.TCPConn.SetKeepAlivePeriod(d)
}
