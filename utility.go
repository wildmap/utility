package utility

import (
	"bytes"
	"reflect"
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

// IMsgID 消息可实现该接口来自定义MsgID，达成如消息结构体复用等高级功能
type IMsgID interface {
	MsgID() uint32
}

// MsgID 求消息的消息ID，传入值必须是指针
func MsgID(m interface{}) uint32 {
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