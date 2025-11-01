package cluster

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/wildmap/utility"
)

const (
	// MsgIDSize 消息ID占用字节数
	MsgIDSize = 8
)

var (
	ErrMsgNotRegistered = errors.New("message not registered")
	ErrMsgRegister      = errors.New("message register failed")
)

// 类型映射表
var (
	id2kind  = make(map[uint64]reflect.Type)
	kind2id  = make(map[reflect.Type]uint64)
	mapMutex sync.RWMutex
)

// Register 注册消息类型（线程安全）
func Register(msg any) error {
	id := utility.MsgID(msg)
	if id <= 0 {
		return ErrMsgRegister
	}
	mapMutex.Lock()
	defer mapMutex.Unlock()

	kind := reflect.TypeOf(msg)
	if kind.Kind() != reflect.Ptr {
		return ErrMsgRegister
	}
	id2kind[id] = kind
	kind2id[kind] = id
	return nil
}

// GetType 根据ID获取类型（线程安全）
func GetType(id uint64) (reflect.Type, bool) {
	mapMutex.RLock()
	defer mapMutex.RUnlock()
	tp, ok := id2kind[id]
	return tp, ok
}

// MarshalPack 打包NATS数据包
// 格式: [MsgID(8字节)][Data(变长)]
func MarshalPack(msg any) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("message is nil")
	}

	// 获取消息类型对应的ID
	kind := reflect.TypeOf(msg)
	mapMutex.RLock()
	id, ok := kind2id[kind]
	mapMutex.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMsgNotRegistered, kind)
	}

	// 序列化消息体
	msgData, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("json marshal failed: %w", err)
	}

	totalSize := MsgIDSize + len(msgData)
	data := make([]byte, totalSize)

	// 写入消息ID（大端序）
	binary.BigEndian.PutUint64(data[0:MsgIDSize], id)

	// 写入消息体
	copy(data[MsgIDSize:], msgData)
	return data, nil
}

// UnmarshalPack 解包NATS数据包
func UnmarshalPack(data []byte) (any, error) {
	// 读取消息ID
	msgID := binary.BigEndian.Uint64(data[0:MsgIDSize])

	// 根据ID获取类型
	mapMutex.RLock()
	kind, ok := id2kind[msgID]
	mapMutex.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: id=%d", ErrMsgNotRegistered, msgID)
	}

	// 创建消息实例
	msgPtr := reflect.New(kind.Elem()).Interface()
	// 反序列化消息体
	if len(data) > MsgIDSize {
		if err := json.Unmarshal(data[MsgIDSize:], msgPtr); err != nil {
			return nil, fmt.Errorf("json unmarshal failed: %w", err)
		}
	}

	return msgPtr, nil
}
