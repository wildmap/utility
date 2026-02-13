package utility

import (
	"strconv"
	"sync"
	"time"

	"github.com/wildmap/utility/xtime"
)

/*
 * uint64 ID生成器
 * 编码格式说明:
 * 42位时间戳(秒级精度) + 21位自增序列号
 * 此设计允许在分布式系统中生成唯一ID
 *
 * 时间范围: 2^42秒 ≈ 139年
 * 每秒容量: 2^21 ≈ 210万个ID
 * 总容量: 2^63-1 个唯一ID
 */

const (
	// timeBits 时间戳位数(42位)
	timeBits uint64 = 42

	// seqBits 序列号位数(21位)
	seqBits uint64 = 21

	// maxTime 可存储的最大时间戳值(2^42 - 1)
	maxTime uint64 = 1<<timeBits - 1

	// maxSeq 最大序列号值(2^21 - 1, 约210万)
	maxSeq uint64 = 1<<seqBits - 1

	// timeShift 时间戳左移位数(21位)
	timeShift uint64 = seqBits

	// seqShift 序列号左移位数(0位,无需移位)
	seqShift uint64 = 0

	// maxId 可生成的最大ID值(2^63 - 1)
	maxId uint64 = (1 << (timeBits + seqBits)) - 1
)

// ID 表示一个唯一标识符,基于int64类型
// 提供了类型转换和提取时间戳/序列号组件的方法
type ID int64

// String 将ID转换为字符串表示
func (i ID) String() string { return strconv.FormatUint(uint64(i), 10) }

// Uint64 将ID转换为无符号64位整数
func (i ID) Uint64() uint64 { return uint64(i) }

// Int64 将ID转换为有符号64位整数
func (i ID) Int64() int64 { return int64(i) }

// Float64 将ID转换为64位浮点数
func (i ID) Float64() float64 {
	return float64(i)
}

// Bytes 将ID转换为其字符串表示的字节切片
func (i ID) Bytes() []byte { return []byte(i.String()) }

// Time 提取并返回ID的时间戳组件(time.Time对象)
// 时间戳存储在ID的高42位中
func (i ID) Time() time.Time {
	t := uint64(i) >> timeShift & maxTime
	return xtime.Sec2Time(int64(t))
}

// Seq 提取并返回ID的序列号组件
// 序列号存储在ID的低21位中
func (i ID) Seq() uint64 {
	return uint64(i) >> seqShift & maxSeq
}

// IDGenerator ID生成器的核心结构
// 维护序列计数器和最后时间戳以确保唯一性
type IDGenerator struct {
	mu        sync.Mutex // 互斥锁,确保线程安全的ID生成
	sequence  uint64     // 当前序列号,每次生成ID时递增
	lastStamp uint64     // 上次使用的时间戳,Unix纪元以来的秒数
}

// NewGenerator 创建并初始化一个新的ID生成器实例
// 将初始序列设置为1并记录当前时间戳
func NewGenerator() *IDGenerator {
	s := &IDGenerator{
		sequence: 1,
	}
	s.lastStamp = s.currentMillis()
	return s
}

// currentMillis 返回当前Unix时间戳(秒)
func (s *IDGenerator) currentMillis() uint64 {
	return uint64(xtime.NowSecTs())
}

// NextID 生成下一个唯一ID
// 通过推进时间戳和重置序列来处理序列溢出
// 通过互斥锁确保线程安全
func (s *IDGenerator) NextID() ID {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查时间戳溢出(超过42位容量)
	if s.lastStamp > maxTime {
		panic("时间戳溢出")
	}

	// 检查序列号溢出(超过21位容量)
	if s.sequence > maxSeq {
		// 等待时间推进到下一秒
		for s.lastStamp > s.currentMillis() {
			time.Sleep(time.Millisecond)
		}
		// 推进时间戳并重置序列
		s.lastStamp++
		s.sequence = 1
	} else {
		// 简单递增序列号
		s.sequence++
	}

	// 将时间戳和序列号组合成最终ID
	// 格式: [42位时间戳][21位序列号]
	return ID(((s.lastStamp << timeShift) | (s.sequence << seqShift)) & maxId)
}

var (
	// idgen 全局单例ID生成器实例
	idgen = NewGenerator()
)

// NextID 使用全局生成器生成一个新的唯一ID
// 这是创建新ID的主要函数
func NextID() ID {
	return idgen.NextID()
}

// ParseID 将uint64值转换为ID类型
// 用于从存储或网络反序列化ID
func ParseID(id uint64) ID {
	return ID(id)
}

// ParseString 将字符串表示转换为ID
// 参数: id - 十进制字符串形式的ID
// 返回: ID实例和可能的解析错误
func ParseString(id string) (ID, error) {
	v, err := strconv.ParseUint(id, 10, 64)
	return ID(v), err
}
