package chanrpc

import (
	"bytes"
	"reflect"
	"sync"
)

// BKDRBytesHash Hash字节序列
func BKDRBytesHash(b []byte) uint32 {
	seed := uint32(131)
	hash := uint32(0)

	for _, v := range b {
		hash = hash*seed + uint32(v)
	}
	return hash
}

// BKDRHash Hash一个字符串
func BKDRHash(s string) uint32 {
	b := bytes.NewBufferString(s).Bytes()
	return BKDRBytesHash(b)
}

// IMessageID 消息可实现该接口来自定义MsgID，达成如消息结构体复用等高级功能
type IMessageID interface {
	MessageID() uint32
}

var (
	msgIDCache sync.Map // map[reflect.Type]uint32
)

// MessageID 求消息的消息ID，传入值必须是指针
func MessageID(m any) uint32 {
	if m == nil {
		return 0
	}

	// 为了安全起见，如果实现了接口，直接调用，不缓存
	if msgIDGen, ok := m.(IMessageID); ok {
		id := msgIDGen.MessageID()
		return id
	}

	typ := reflect.TypeOf(m)
	if typ.Kind() == reflect.Ptr {
		// 处理指针类型，获取其底层元素的类型名称
		typ = typ.Elem()
	}
	// 优先检查缓存
	if v, ok := msgIDCache.Load(typ); ok {
		return v.(uint32)
	}

	// 使用包含包名的完整路径 String() 返回如 "package.StructName"
	name := typ.String()
	id := BKDRHash(name)

	msgIDCache.Store(typ, id)
	return id
}
