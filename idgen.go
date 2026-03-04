package utility

import (
	"strconv"
	"sync"
	"time"

	"github.com/wildmap/utility/xtime"
)

// ID 生成器——基于改进的 Snowflake 算法。
//
// 编码格式（共 63 位有效位）：
//
//	[42位秒级时间戳 | 21位自增序列号]
//
// 容量指标：
//   - 时间跨度：2^42 秒 ≈ 139 年（从 Unix 纪元起算）
//   - 每秒容量：2^21 ≈ 210 万个唯一 ID
//   - 总容量：2^63 - 1 个不重复 ID
//
// 与标准 Snowflake 的区别：
//   - 使用秒级而非毫秒级时间戳，牺牲部分时间精度换取更大的序列号空间
//   - 21 位序列号（vs 标准 12 位），每秒支持约 210 万 ID（vs 4096 个）
//   - 适合单机高并发场景，分布式场景需额外引入机器 ID 字段

const (
	timeBits  uint64 = 42                              // 时间戳占位数
	seqBits   uint64 = 21                              // 序列号占位数
	maxTime   uint64 = 1<<timeBits - 1                 // 时间戳最大值（2^42 - 1）
	maxSeq    uint64 = 1<<seqBits - 1                  // 序列号最大值（2^21 - 1，约 210 万）
	timeShift uint64 = seqBits                         // 时间戳在 ID 中的左移位数
	seqShift  uint64 = 0                               // 序列号占据低位，无需移位
	maxId     uint64 = (1 << (timeBits + seqBits)) - 1 // ID 最大值（2^63 - 1）
)

// ID 表示一个全局唯一标识符，底层类型为 int64。
//
// 提供类型转换方法和从 ID 中提取时间戳、序列号的分析方法，
// 便于在排查问题时反解 ID 的生成时间和序列信息。
type ID int64

// String 返回 ID 的十进制字符串表示，便于 JSON 序列化和日志输出。
func (i ID) String() string { return strconv.FormatUint(uint64(i), 10) }

// Uint64 将 ID 转换为无符号 64 位整数，适合位运算或无符号数值处理场景。
func (i ID) Uint64() uint64 { return uint64(i) }

// Int64 将 ID 转换为有符号 64 位整数，适合存入数据库的 bigint 字段。
func (i ID) Int64() int64 { return int64(i) }

// Float64 将 ID 转换为 64 位浮点数，注意大整数转 float64 可能有精度损失。
func (i ID) Float64() float64 {
	return float64(i)
}

// Bytes 返回 ID 十进制字符串的字节切片，适合写入 []byte 类型的字段。
func (i ID) Bytes() []byte { return []byte(i.String()) }

// Time 从 ID 中提取并返回其生成时的时间戳（time.Time 对象）。
//
// 通过右移 timeShift 位取出高 42 位时间戳，再通过掩码确保仅取 42 位有效值，
// 最后转换为 time.Time，可用于调试、日志分析和按时间范围过滤 ID。
func (i ID) Time() time.Time {
	t := uint64(i) >> timeShift & maxTime
	return xtime.Sec2Time(int64(t))
}

// Seq 从 ID 中提取并返回其序列号部分（低 21 位）。
//
// 序列号在同一秒内单调递增，可用于判断 ID 的生成顺序。
func (i ID) Seq() uint64 {
	return uint64(i) >> seqShift & maxSeq
}

// IDGenerator 线程安全的 ID 生成器，维护序列计数器和最后时间戳。
//
// 并发安全说明：通过 sync.Mutex 保护内部状态，所有并发调用均需获取锁，
// 确保同一时间戳内的序列号单调递增且不重复。
type IDGenerator struct {
	mu        sync.Mutex // 保护 sequence 和 lastStamp，确保多 goroutine 下的唯一性
	sequence  uint64     // 当前秒内的自增序列号（从 1 开始）
	lastStamp uint64     // 最后一次使用的秒级时间戳
}

// NewGenerator 创建并初始化 ID 生成器，记录当前时间戳作为起点。
func NewGenerator() *IDGenerator {
	s := &IDGenerator{
		sequence: 1,
	}
	s.lastStamp = s.currentMillis()
	return s
}

// currentMillis 获取当前 Unix 秒级时间戳。
func (s *IDGenerator) currentMillis() uint64 {
	return uint64(xtime.NowSecTs())
}

// NextID 生成下一个全局唯一 ID，线程安全。
//
// 序列号溢出处理策略：
// 当同一秒内序列号耗尽（超过 2^21）时，主动等待时间推进至下一秒，
// 而非回绕到 0，确保不同秒之间的 ID 不会出现重叠。
//
// 时间戳溢出（约 139 年后）将触发 panic，这是有意为之的防御性设计，
// 届时系统早应升级或迁移。
func (s *IDGenerator) NextID() ID {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 时间戳超过 42 位最大值（约 2163 年），直接 panic 防止生成无效 ID
	if s.lastStamp > maxTime {
		panic("时间戳溢出")
	}

	if s.sequence > maxSeq {
		// 序列号耗尽：自旋等待时间跨越到下一秒，避免序列号回绕导致重复
		for s.lastStamp > s.currentMillis() {
			time.Sleep(time.Millisecond)
		}
		s.lastStamp++
		s.sequence = 1
	} else {
		s.sequence++
	}

	// 组合 ID：将时间戳左移 21 位后与序列号按位或，最后与 maxId 掩码确保 63 位有效
	return ID(((s.lastStamp << timeShift) | (s.sequence << seqShift)) & maxId)
}

var (
	// idgen 包级全局单例 ID 生成器，供 NextID 函数使用。
	idgen = NewGenerator()
)

// NextID 使用全局生成器生成一个新的唯一 ID，是通常情况下的推荐调用方式。
func NextID() ID {
	return idgen.NextID()
}

// ParseID 将 uint64 值转换为 ID 类型，用于从存储层或网络层反序列化 ID。
func ParseID(id uint64) ID {
	return ID(id)
}

// ParseString 将十进制字符串解析为 ID 类型，用于从 HTTP 请求参数等场景解析 ID。
func ParseString(id string) (ID, error) {
	v, err := strconv.ParseUint(id, 10, 64)
	return ID(v), err
}
