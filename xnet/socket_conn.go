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
	// MsgLenSize 消息长度字段占用的字节数（4 字节 uint32，大端序）。
	// 单条消息最大支持 4GB，实际受 MaxMsgLen（50MB）限制。
	MsgLenSize = 4
)

// SocketConn 封装 net.Conn，实现基于长度前缀的消息帧协议。
//
// 消息格式：[4字节大端序消息长度][消息体]
//
// 设计考量：
//   - TCP 是字节流协议，不保留应用层消息边界，长度前缀是最简单可靠的分帧方案
//   - 写操作通过 writeMutex 保证并发安全，读操作通常由单一 goroutine 执行无需加锁
//   - 使用 io.ReadFull 确保读取完整数据，防止网络层分包导致数据截断
type SocketConn struct {
	conn       net.Conn   // 底层 TCP/KCP 网络连接
	writeMutex sync.Mutex // 保护写操作的并发安全（多 goroutine 同时写入会导致消息交叉）
}

// NewSocketConn 封装 net.Conn 为带消息帧功能的 SocketConn。
func NewSocketConn(conn net.Conn) *SocketConn {
	return &SocketConn{
		conn: conn,
	}
}

// Close 关闭底层网络连接，释放文件描述符。
func (t *SocketConn) Close() error {
	return t.conn.Close()
}

// RemoteAddr 返回远端（客户端）的网络地址。
func (t *SocketConn) RemoteAddr() net.Addr {
	return t.conn.RemoteAddr()
}

// LocalAddr 返回本端（服务器）的网络地址。
func (t *SocketConn) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

// ReadMsg 从 TCP 连接读取一条完整的应用层消息。
//
// 读取流程：
//  1. ReadFull 读取 4 字节长度头（确保完整读取，不受 TCP 分包影响）
//  2. BigEndian 解析消息长度（网络字节序）
//  3. 校验长度合法性（非零且不超过 MaxMsgLen）
//  4. ReadFull 读取消息体
//
// 阻塞特性：连接无数据时阻塞等待，连接关闭时返回 io.EOF 或相关错误。
func (t *SocketConn) ReadMsg() ([]byte, error) {
	var msgLenBuf [MsgLenSize]byte
	if _, err := io.ReadFull(t.conn, msgLenBuf[:]); err != nil {
		return nil, fmt.Errorf("read msg length failed: %w", err)
	}

	// 大端序（网络字节序）解析长度，与跨平台客户端兼容
	msgLen := binary.BigEndian.Uint32(msgLenBuf[:])

	if msgLen == 0 || msgLen > MaxMsgLen {
		return nil, errors.New("read msg length out of range")
	}

	msgData := make([]byte, msgLen)
	if _, err := io.ReadFull(t.conn, msgData[:msgLen]); err != nil {
		return nil, fmt.Errorf("read msg body failed: %w", err)
	}

	return msgData, nil
}

// WriteMsg 向 TCP 连接写入一条消息，线程安全。
//
// 构造完整帧（[4字节长度][消息体]）后一次性写入，
// 避免分两次 Write 导致 Nagle 算法延迟或中间插入其他 goroutine 的消息。
// 通过 writeMutex 保证并发安全，多个 goroutine 可安全并发调用。
func (t *SocketConn) WriteMsg(data []byte) error {
	t.writeMutex.Lock()
	defer t.writeMutex.Unlock()

	msgLen := uint32(len(data))

	if msgLen == 0 {
		return nil
	}

	// 预分配连续内存，将长度头和消息体合并为一次 Write 调用，减少系统调用次数
	msg := make([]byte, MsgLenSize+msgLen)
	binary.BigEndian.PutUint32(msg[:MsgLenSize], msgLen)
	copy(msg[MsgLenSize:], data)

	if _, err := t.conn.Write(msg); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}
