package utility

import (
	"bytes"
	"cmp"
	"math/rand/v2"
	"reflect"
)

func SliceDiff[V cmp.Ordered](a, b []V) []V {
	var diff []V
	for _, v := range a {
		if FindIdx(b, v) < 0 {
			diff = append(diff, v)
		}
	}
	return diff
}

// RandInterval 在[b1,b2]区间内随机一次
func RandInterval(b1, b2 int32) int32 {
	if b1 == b2 {
		return b1
	}

	_min, _max := int64(b1), int64(b2)
	if _min > _max {
		_min, _max = _max, _min
	}
	return int32(rand.Int64N(_max-_min+1) + _min)
}

// RandIntervalN 在[b1, b2]的区间内随机N次
func RandIntervalN(b1, b2 int32, n uint32) []int32 {
	if b1 == b2 {
		return []int32{b1}
	}

	_min, _max := int64(b1), int64(b2)
	if _min > _max {
		_min, _max = _max, _min
	}
	l := _max - _min + 1
	if int64(n) > l {
		n = uint32(l)
	}

	r := make([]int32, n)
	m := make(map[int32]int32)
	for i := uint32(0); i < n; i++ {
		v := int32(rand.Int64N(l) + _min)

		if mv, ok := m[v]; ok {
			r[i] = mv
		} else {
			r[i] = v
		}

		lv := int32(l - 1 + _min)
		if v != lv {
			if mv, ok := m[lv]; ok {
				m[v] = mv
			} else {
				m[v] = lv
			}
		}

		l--
	}

	return r
}

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
func MsgID(m any) uint32 {
	typ := reflect.TypeOf(m)
	// 处理 nil 消息
	if typ == nil {
		return 0
	}
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