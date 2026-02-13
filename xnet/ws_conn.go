package xnet

import (
	"net"
	"sync"

	"github.com/gorilla/websocket"
)

// WSConn 封装gorilla/websocket连接以实现IConn接口
// 为WebSocket连接提供线程安全的消息读写功能
type WSConn struct {
	conn       *websocket.Conn // 底层WebSocket连接
	remoteAddr net.Addr        // 真实远程地址(来自代理头或直接连接)
	writeMutex sync.Mutex      // 互斥锁,确保写入操作的线程安全
}

// NewWSConn 创建一个新的WSConn实例,封装给定的WebSocket连接
// 参数: conn - WebSocket连接, remoteAddr - 远程地址(为nil时使用conn的RemoteAddr)
// 返回: WSConn实例指针
func NewWSConn(conn *websocket.Conn, remoteAddr net.Addr) *WSConn {
	if remoteAddr == nil {
		remoteAddr = conn.RemoteAddr()
	}
	return &WSConn{
		conn:       conn,
		remoteAddr: remoteAddr,
	}
}

// Close 关闭WebSocket连接
func (c *WSConn) Close() error {
	return c.conn.Close()
}

// RemoteAddr 返回远程网络地址
func (c *WSConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// LocalAddr 返回本地网络地址
func (c *WSConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// ReadMsg 从WebSocket连接读取一条消息
// 返回: 消息数据和可能的错误
func (c *WSConn) ReadMsg() ([]byte, error) {
	_, data, err := c.conn.ReadMessage()
	return data, err
}

// WriteMsg 以线程安全的方式向WebSocket连接写入文本消息
// 参数: data - 要写入的消息字节数组
// 返回: 写入失败时的错误
func (c *WSConn) WriteMsg(data []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	err := c.conn.WriteMessage(websocket.TextMessage, data)
	return err
}
