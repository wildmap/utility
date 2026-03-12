package chanrpc

import (
	"bytes"
	"reflect"
	"sync"
)

// BKDRBytesHash 使用 BKDR 哈希算法计算字节序列的哈希值。
//
// BKDR 算法以 131 为种子，通过多项式滚动哈希生成 uint32 值。
// 选用 131 作为种子是因为其在字符串哈希场景中具有良好的雪崩效应和均匀分布特性，
// 实践中碰撞率极低，非常适合用于消息类型 ID 的生成。
func BKDRBytesHash(b []byte) uint32 {
	seed := uint32(131)
	hash := uint32(0)

	for _, v := range b {
		hash = hash*seed + uint32(v)
	}
	return hash
}

// BKDRHash 计算字符串的 BKDR 哈希值，内部转换为字节序列后复用 BKDRBytesHash。
func BKDRHash(s string) uint32 {
	b := bytes.NewBufferString(s).Bytes()
	return BKDRBytesHash(b)
}

// IMessageID 允许消息结构体自定义其消息 ID 的接口。
//
// 默认策略通过反射获取类型全限定名再做哈希，但存在两种场景需要自定义 ID：
//  1. 同一结构体在不同上下文中表示不同语义（复用结构体降低内存分配）
//  2. 需要与外部协议 ID 对齐（如 protobuf 消息号）
//
// 实现此接口的消息不使用反射缓存，每次调用均直接执行，调用方应保证 ID 全局唯一且稳定。
type IMessageID interface {
	MessageID() uint32
}

var (
	// msgIDCache 缓存已通过反射计算过的消息类型 → ID 映射，避免重复的反射和哈希计算。
	// 选用 sync.Map 而非加锁 map，是因为消息类型集合在运行期趋于稳定（写少读多），
	// sync.Map 的无锁读路径在此场景下显著优于 RWMutex。
	msgIDCache sync.Map // map[reflect.Type]uint32
)

// MessageID 根据消息对象的类型推导其全局唯一消息 ID。
//
// 计算策略（优先级从高到低）：
//  1. 若消息实现了 IMessageID 接口，直接调用其 MessageID() 方法（跳过缓存）
//  2. 否则，基于消息类型的包含包名的完整类型字符串（如 "mypkg.MyMsg"）计算 BKDR 哈希，
//     结果存入 sync.Map 缓存，后续同类型消息直接命中缓存，无需再次反射
//
// 指针类型自动解引用为元素类型（*T → T），保证 T 和 *T 共享同一个 ID，
// 消除调用方因传值/传指针不一致导致路由失败的隐患。
func MessageID(m any) uint32 {
	if m == nil {
		return 0
	}

	// 优先使用接口自定义 ID，绕过反射缓存，保证接口实现方的灵活性
	if msgIDGen, ok := m.(IMessageID); ok {
		id := msgIDGen.MessageID()
		return id
	}

	typ := reflect.TypeOf(m)
	if typ.Kind() == reflect.Pointer {
		// 解引用指针，确保 *MyMsg 与 MyMsg 映射到同一个 Handler，避免注册/调用类型不匹配
		typ = typ.Elem()
	}

	// 优先命中缓存，避免重复的 reflect.Type.String() 调用和哈希运算
	if v, ok := msgIDCache.Load(typ); ok {
		return v.(uint32)
	}

	// typ.String() 返回包含包路径的完整类型标识（如 "chanrpc.CallInfo"），
	// 相比仅用简短类型名，有效规避不同包中同名结构体产生 ID 碰撞的问题
	name := typ.String()
	id := BKDRHash(name)

	msgIDCache.Store(typ, id)
	return id
}
