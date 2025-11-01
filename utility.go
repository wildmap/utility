package utility

import (
	"bytes"
	"reflect"
)

// BKDRBytesHash Hash字节序列
func BKDRBytesHash(b []byte) uint64 {
	seed := uint64(131)
	hash := uint64(0)

	for _, v := range b {
		hash = hash*seed + uint64(v)
	}
	return hash
}

// BKDRHash Hash一个字符串
func BKDRHash(s string) uint64 {
	b := bytes.NewBufferString(s).Bytes()
	return BKDRBytesHash(b)
}

// IMsgID 消息可实现该接口来自定义MsgID，达成如消息结构体复用等高级功能
type IMsgID interface {
	MsgID() uint64
}

// MsgID 求消息的消息ID，传入值必须是指针
func MsgID(m interface{}) uint64 {
	typ := reflect.TypeOf(m)
	if msgIDGen, ok := m.(IMsgID); ok {
		return msgIDGen.MsgID()
	}
	if typ.Kind() == reflect.Struct {
		return BKDRHash(typ.Name())
	}
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return BKDRHash(typ.Name())
}