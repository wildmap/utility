package xnet

import (
	"context"
	"net"
)

// IConn 定义网络连接的统一接口，屏蔽底层协议（TCP/WebSocket）的差异。
//
// 通过此接口，业务代码（IAgent）可以透明地处理不同协议的连接，
// 无需关心消息的帧格式（TCP 的长度前缀帧 vs WebSocket 的内置帧格式）。
type IConn interface {
	// ReadMsg 从连接中读取一条完整的应用层消息，阻塞直到消息到达或连接关闭。
	ReadMsg() ([]byte, error)

	// WriteMsg 向连接写入一条完整的应用层消息，线程安全。
	WriteMsg(args []byte) error

	// LocalAddr 返回本地端的网络地址。
	LocalAddr() net.Addr

	// RemoteAddr 返回远端的网络地址（客户端 IP:Port）。
	RemoteAddr() net.Addr

	// Close 关闭连接，释放底层网络资源，所有阻塞的读写操作将立即返回错误。
	Close() error
}

// IAgent 定义处理单个客户端连接的代理接口。
//
// 框架为每个新连接创建一个 IAgent 实例，严格按照 OnInit → Run → OnClose 的顺序调用，
// 调用方（Server.handleConn）保证生命周期方法在同一 goroutine 中串行执行（Run 除外）。
//
// 实现建议：
//   - OnInit 中初始化连接相关的资源（鉴权、注册等）
//   - Run 中循环读取消息并处理，直到连接关闭
//   - OnClose 中释放连接相关的资源（注销、清理等）
type IAgent interface {
	// OnInit 连接建立后的初始化回调，返回 error 将立即关闭连接并调用 OnClose。
	OnInit(ctx context.Context) error

	// Run 连接的消息处理主循环，阻塞直到连接关闭或发生不可恢复的错误。
	Run(ctx context.Context)

	// OnClose 连接关闭后的清理回调，无论 OnInit 是否成功都会调用。
	OnClose(ctx context.Context)
}
