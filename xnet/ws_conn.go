package xnet

import (
	"net"
	"sync"

	"github.com/gorilla/websocket"
)

// WSConn 封装 gorilla/websocket.Conn，实现 IConn 接口。
//
// WebSocket 自带消息帧协议，无需像 TCP 那样手动处理长度前缀，
// 读写操作直接对应完整的 WebSocket 消息帧，简化了应用层协议设计。
//
// 并发安全：gorilla/websocket 要求写操作串行化，
// 通过 writeMutex 保证多 goroutine 并发写入时不产生帧交叉。
// 读操作通常由单一 goroutine 执行，无需额外加锁。
type WSConn struct {
	conn       *websocket.Conn // 底层 WebSocket 连接
	remoteAddr net.Addr        // 客户端真实 IP 地址（可能来自代理头，非 conn.RemoteAddr）
	writeMutex sync.Mutex      // 保护写操作的并发安全
}

// NewWSConn 创建 WSConn 实例。
//
// 当 remoteAddr 为 nil 时，回退使用 conn.RemoteAddr（即代理服务器 IP），
// 传入非 nil 的 remoteAddr 用于设置从请求头中提取的真实客户端 IP。
func NewWSConn(conn *websocket.Conn, remoteAddr net.Addr) *WSConn {
	if remoteAddr == nil {
		remoteAddr = conn.RemoteAddr()
	}
	return &WSConn{
		conn:       conn,
		remoteAddr: remoteAddr,
	}
}

// Close 关闭 WebSocket 连接。
func (c *WSConn) Close() error {
	return c.conn.Close()
}

// RemoteAddr 返回客户端的真实网络地址。
func (c *WSConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// LocalAddr 返回服务器端的网络地址。
func (c *WSConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// ReadMsg 读取一条完整的 WebSocket 消息，阻塞直到消息到达或连接关闭。
//
// WebSocket 帧协议保证 ReadMessage 返回的是完整的应用层消息，
// 无需像 TCP 那样手动处理粘包和拆包问题。
func (c *WSConn) ReadMsg() ([]byte, error) {
	_, data, err := c.conn.ReadMessage()
	return data, err
}

// WriteMsg 以文本类型（TextMessage）发送一条 WebSocket 消息，线程安全。
//
// 使用 TextMessage 而非 BinaryMessage 是一种常见选择，
// 在实际场景中两者均可工作，客户端收到的都是字节数组。
// 通过 writeMutex 确保并发写入时消息帧不交叉。
func (c *WSConn) WriteMsg(data []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	err := c.conn.WriteMessage(websocket.TextMessage, data)
	return err
}
