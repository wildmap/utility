package xnet

import (
	"context"
	"net"
)

// IConn 定义网络连接的接口
// 提供读写消息和访问连接地址的方法
type IConn interface {
	// ReadMsg 从连接中读取一条完整的消息
	// 返回: 消息数据或读取失败时的错误
	ReadMsg() ([]byte, error)

	// WriteMsg 向连接写入一条完整的消息
	// 参数: args - 要写入的消息字节数组
	// 返回: 写入失败时的错误
	WriteMsg(args []byte) error

	// LocalAddr 返回本地网络地址
	LocalAddr() net.Addr

	// RemoteAddr 返回远程网络地址
	RemoteAddr() net.Addr

	// Close 关闭连接
	// 任何被阻塞的读写操作将被解除阻塞并返回错误
	Close() error
}

// IAgent 定义处理客户端连接的网络代理接口
// 实现类必须提供初始化、主处理循环和清理逻辑
type IAgent interface {
	// OnInit 在建立连接时执行初始化任务
	// 参数: ctx - 上下文对象
	// 返回: 初始化失败时的错误
	OnInit(ctx context.Context) error

	// Run 执行处理客户端请求的主循环
	// 该方法会阻塞直到连接关闭或发生错误
	// 参数: ctx - 上下文对象
	Run(ctx context.Context)

	// OnClose 在连接关闭时执行清理任务
	// 参数: ctx - 上下文对象
	OnClose(ctx context.Context)
}
