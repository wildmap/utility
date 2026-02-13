package utility

import (
	"cmp"
	"math/rand/v2"
	"strings"
)

// ToCamelCase 通用的 snake_case 到 CamelCase 转换
func ToCamelCase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	n := strings.Builder{}
	n.Grow(len(s))
	capNext := true
	prevIsCap := false
	for i, v := range []byte(s) {
		vIsCap := v >= 'A' && v <= 'Z'
		vIsLow := v >= 'a' && v <= 'z'
		if capNext {
			if vIsLow {
				v += 'A'
				v -= 'a'
			}
		} else if i == 0 {
			if vIsCap {
				v += 'a'
				v -= 'A'
			}
		} else if prevIsCap && vIsCap {
			v += 'a'
			v -= 'A'
		}
		prevIsCap = vIsCap

		if vIsCap || vIsLow {
			n.WriteByte(v)
			capNext = false
		} else if vIsNum := v >= '0' && v <= '9'; vIsNum {
			n.WriteByte(v)
			capNext = true
		} else {
			capNext = v == '_' || v == ' ' || v == '-' || v == '.'
		}
	}
	return n.String()
}

// SliceDiff 计算切片a中存在但b中不存在的元素(差集)
// 参数: a - 第一个切片, b - 第二个切片
// 返回: 只在a中存在的元素切片
func SliceDiff[V cmp.Ordered](a, b []V) []V {
	var diff []V
	for _, v := range a {
		if FindIdx(b, v) < 0 {
			diff = append(diff, v)
		}
	}
	return diff
}

// RandInterval 在[b1,b2]区间内生成一个随机整数
// 参数: b1 - 区间下界, b2 - 区间上界
// 返回: [b1,b2]区间内的随机int32值
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

// RandIntervalN 在[b1, b2]区间内不重复地随机选择N个整数
// 使用Fisher-Yates洗牌算法的优化版本
// 参数: b1 - 区间下界, b2 - 区间上界, n - 要选择的数量
// 返回: 包含n个不重复随机数的切片
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
