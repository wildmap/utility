package utility

import (
	"cmp"
	"math/rand/v2"
	"strings"
)

// ToCamelCase 将 snake_case 或其他分隔符风格的字符串转换为 CamelCase（大驼峰）。
//
// 支持的分隔符：下划线（_）、空格（ ）、连字符（-）、点号（.）。
// 算法逐字节扫描，通过 capNext 标志位控制下一个字母是否需要大写：
//   - 连续大写字母序列（如 "XML"）会被规范化（"Xml"），防止与 Go 的命名风格冲突
//   - 数字后的字母会自动大写（视为新单词的开始）
func ToCamelCase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	n := strings.Builder{}
	n.Grow(len(s)) // 预分配与原字符串等长的缓冲区，避免扩容
	capNext := true
	prevIsCap := false
	for i, v := range []byte(s) {
		vIsCap := v >= 'A' && v <= 'Z'
		vIsLow := v >= 'a' && v <= 'z'
		if capNext {
			// 当前位置需要大写：将小写字母转为大写
			if vIsLow {
				v += 'A'
				v -= 'a'
			}
		} else if i == 0 {
			// 首字母特殊处理：若首字母是大写，转为小写（CamelCase 首字母大写由 capNext 初始值控制）
			if vIsCap {
				v += 'a'
				v -= 'A'
			}
		} else if prevIsCap && vIsCap {
			// 连续大写序列（如 "XML"）中的非首字母统一转小写，避免全大写缩写干扰驼峰转换
			v += 'a'
			v -= 'A'
		}
		prevIsCap = vIsCap

		if vIsCap || vIsLow {
			n.WriteByte(v)
			capNext = false
		} else if vIsNum := v >= '0' && v <= '9'; vIsNum {
			// 数字直接保留，并将 capNext 置 true，使数字后的字母大写
			n.WriteByte(v)
			capNext = true
		} else {
			// 遇到分隔符（_、空格、-、.）时设置 capNext，其余特殊字符忽略
			capNext = v == '_' || v == ' ' || v == '-' || v == '.'
		}
	}
	return n.String()
}

// SliceDiff 计算切片 a 相对于切片 b 的差集，即仅在 a 中存在、b 中不存在的元素集合。
//
// 时间复杂度：O(n*m)，其中 n 和 m 分别为 a 和 b 的长度。
// 对于大规模数据，建议先将 b 转换为 map 以将复杂度降至 O(n+m)。
func SliceDiff[V cmp.Ordered](a, b []V) []V {
	var diff []V
	for _, v := range a {
		if FindIdx(b, v) < 0 {
			diff = append(diff, v)
		}
	}
	return diff
}

// RandInterval 在闭区间 [b1, b2] 内生成一个均匀分布的随机 int32 整数。
//
// 自动处理 b1 > b2 的情况（交换边界），因此参数顺序不影响结果。
// 当 b1 == b2 时直接返回该值，避免无意义的随机数计算。
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

// RandIntervalN 在闭区间 [b1, b2] 内不重复地随机抽取 n 个整数。
//
// 基于 Knuth 洗牌算法（Fisher-Yates）的空间优化变体（Sattolo 算法思想）：
// 使用 map 记录已"虚拟交换"的位置，无需实际构建完整数组，
// 时间复杂度 O(n)，空间复杂度 O(n)（n 远小于区间长度时效率极高）。
//
// 当 n 超过区间长度时，自动截断为区间内所有整数。
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
	// m 模拟"虚拟数组"的交换状态：m[v] 表示位置 v 实际存储的值（若不在 map 中则为 v 本身）
	m := make(map[int32]int32)
	for i := uint32(0); i < n; i++ {
		// 在 [_min, _min+l-1] 范围内随机选一个尚未被选过的位置
		v := int32(rand.Int64N(l) + _min)

		// 若位置 v 已被"虚拟交换"，取其映射值；否则直接使用 v
		if mv, ok := m[v]; ok {
			r[i] = mv
		} else {
			r[i] = v
		}

		// 将末尾位置的值"虚拟交换"到位置 v（模拟 Fisher-Yates 的交换步骤）
		lv := int32(l - 1 + _min)
		if v != lv {
			if mv, ok := m[lv]; ok {
				m[v] = mv
			} else {
				m[v] = lv
			}
		}

		l-- // 缩小可选范围，确保下次不会再选到已选过的位置
	}

	return r
}
