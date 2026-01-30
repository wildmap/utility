package xnet

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

const (
	// MsgLenSize 消息长度字段的字节大小(4字节,用于uint32)
	MsgLenSize = 4
)

// SocketConn 封装net.Conn以提供带长度前缀的消息读写功能
// 消息格式: [4字节长度][消息体]
// 通过互斥锁确保并发写入的线程安全性
type SocketConn struct {
	conn       net.Conn   // 底层网络连接
	writeMutex sync.Mutex // 互斥锁,确保写入操作的线程安全
}

// NewSocketConn 创建一个新的SocketConn实例,封装给定的net.Conn
// 参数: conn - 底层网络连接
// 返回: SocketConn实例指针
func NewSocketConn(conn net.Conn) *SocketConn {
	return &SocketConn{
		conn: conn,
	}
}

// Close 关闭底层网络连接
func (t *SocketConn) Close() error {
	return t.conn.Close()
}

// RemoteAddr 返回远程网络地址
func (t *SocketConn) RemoteAddr() net.Addr {
	return t.conn.RemoteAddr()
}

// LocalAddr 返回本地网络地址
func (t *SocketConn) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

// ReadMsg 从TCP连接读取带长度前缀的消息
// 消息格式: [4字节大端序长度][消息体]
// 返回: 消息数据和可能的错误
func (t *SocketConn) ReadMsg() ([]byte, error) {
	// 步骤1: 读取4字节的消息长度头
	var msgLenBuf [MsgLenSize]byte
	if _, err := io.ReadFull(t.conn, msgLenBuf[:]); err != nil {
		return nil, fmt.Errorf("read msg length failed: %w", err)
	}

	// 步骤2: 解析消息长度(大端序)
	msgLen := binary.BigEndian.Uint32(msgLenBuf[:])

	// 步骤3: 验证消息长度
	// 确保长度不为0且不超过最大限制
	if msgLen == 0 || msgLen > MaxMsgLen {
		return nil, errors.New("read msg length out of range")
	}

	// 步骤4: 读取消息体
	msgData := make([]byte, msgLen)
	if _, err := io.ReadFull(t.conn, msgData[:msgLen]); err != nil {
		return nil, fmt.Errorf("read msg body failed: %w", err)
	}

	return msgData, nil
}

// WriteMsg 向TCP连接写入带长度前缀的消息
// 消息格式: [4字节大端序长度][消息体]
// 该方法是线程安全的,可以并发调用
// 参数: data - 要写入的消息字节数组
// 返回: 写入失败时的错误
func (t *SocketConn) WriteMsg(data []byte) error {
	t.writeMutex.Lock()
	defer t.writeMutex.Unlock()

	msgLen := uint32(len(data))

	// 如果消息为空则跳过
	if msgLen == 0 {
		return nil
	}

	// 构造完整消息: [4字节长度][消息体]
	msg := make([]byte, MsgLenSize+msgLen)
	binary.BigEndian.PutUint32(msg[:MsgLenSize], msgLen)
	copy(msg[MsgLenSize:], data)

	// 写入TCP连接
	if _, err := t.conn.Write(msg); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}
