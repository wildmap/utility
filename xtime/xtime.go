package xtime

import (
	"sync/atomic"
	"time"
)

// 常用时间单位的毫秒换算常量。
const (
	SecMs  = 1000        // 1 秒 = 1000 毫秒
	MinMs  = 60 * SecMs  // 1 分钟 = 60 秒
	HourMs = 60 * MinMs  // 1 小时 = 60 分钟
	DayMs  = 24 * HourMs // 1 天 = 24 小时
)

// 时间格式化模板常量，遵循 Go 的时间格式规范（以 2006-01-02 15:04:05 为参照时刻）。
const (
	StdTimeFormat       = "2006-01-02 15:04:05.999" // 含毫秒的标准格式
	StdTimeFormatMinute = "2006-01-02 15:04"        // 精确到分钟的格式
	DefaultTimeFormat   = "2006-01-02 15:04:05"     // 默认日期时间格式
)

var (
	// LocalTime 控制时间函数返回本地时间还是 UTC 时间。
	// false（默认）表示使用 UTC，设置为 true 时使用系统本地时区。
	LocalTime bool = false

	// TimeZoneName 当前系统本地时区名称（如 "CST"），在 init 中初始化。
	TimeZoneName = ""

	// TimeZoneOffset 当前系统本地时区偏移量（秒），在 init 中初始化。
	TimeZoneOffset = 0

	// useOffset 是否启用逻辑时间偏移，用于测试和调试。
	// 默认关闭，仅在测试场景下开启，不影响生产环境时间精度。
	useOffset = false

	// offset 逻辑时间偏移量，使用 atomic.Int64 保证并发安全的读写。
	// 存储 time.Duration 类型的纳秒值。
	offset atomic.Int64
)

func init() {
	// 在包加载时获取并缓存本地时区信息，避免每次调用 Now 都执行系统调用
	TimeZoneName, TimeZoneOffset = time.Now().Local().Zone()
}

// Parse 将字符串解析为 time.Time，格式为 "2006-01-02 15:04:05"。
//
// 根据 LocalTime 标志决定解析为本地时间还是 UTC 时间，
// 保证 Parse 与 Now 的时区语义一致。
func Parse(timeStr string) (time.Time, error) {
	if LocalTime {
		return time.ParseInLocation(DefaultTimeFormat, timeStr, time.Local)
	}
	return time.Parse(DefaultTimeFormat, timeStr)
}

// SetLocalTime 设置是否使用本地时间模式。
func SetLocalTime(localtime bool) {
	LocalTime = localtime
}

// SetUseOffset 开启或关闭逻辑时间偏移功能。
//
// 生产环境应保持关闭，仅在测试"时间相关逻辑"时开启，
// 通过 SetOffset 或 AddOffset 调整逻辑时间，无需修改系统时钟。
func SetUseOffset(use bool) {
	useOffset = use
}

// SetOffset 设置绝对时间偏移量，覆盖之前的偏移值。
func SetOffset(dur time.Duration) {
	offset.Store(int64(dur))
}

// AddOffset 在现有偏移量基础上累加时间偏移。
//
// 使用 atomic.Int64 的 Load + Store 实现，在高并发调用时存在非原子读-改-写的问题，
// 但时间偏移通常在测试初始化阶段单线程设置，此处不使用 CAS 是合理的权衡。
func AddOffset(dur time.Duration) {
	offset.Store(offset.Load() + int64(dur))
}

// ChangeTimeTo 将逻辑时间调整到指定目标时间。
//
// 计算当前真实时间与目标时间的差值并设为偏移量，
// 此后 Now() 返回的时间将接近目标时间（随真实时间流逝）。
func ChangeTimeTo(t time.Time) {
	dur := t.Sub(ToUTC(time.Now()))
	SetOffset(dur)
}

// ClearOffset 清除时间偏移，恢复为真实系统时间。
func ClearOffset() {
	offset.Store(0)
}

// GetOffset 获取当前设置的时间偏移量。
func GetOffset() time.Duration {
	return time.Duration(offset.Load())
}

// Now 获取当前逻辑时间（考虑时区设置和时间偏移）。
//
// 是整个 xtime 包的核心函数，所有时间戳获取函数都基于此函数实现。
// useOffset=true 时，在真实时间上叠加 offset，实现逻辑时间控制。
func Now() time.Time {
	now := ToUTC(time.Now())
	if useOffset {
		return now.Add(time.Duration(offset.Load()))
	}
	return now
}

// NowSecTs 获取当前秒级 Unix 时间戳。
func NowSecTs() int64 {
	return Now().Unix()
}

// NowTs 获取当前毫秒级 Unix 时间戳。
func NowTs() int64 {
	return Now().UnixMilli()
}

// NowUsTs 获取当前微秒级 Unix 时间戳。
func NowUsTs() int64 {
	return Now().UnixMicro()
}

// ToUTC 根据 LocalTime 配置决定保留本地时间或转换为 UTC。
//
// 统一的时区处理入口，保证所有时间转换逻辑一致。
func ToUTC(t time.Time) time.Time {
	if LocalTime {
		return t
	}
	return t.UTC()
}

// Time2Ms 将 time.Time 转换为毫秒级 Unix 时间戳。
func Time2Ms(t time.Time) int64 {
	return ToUTC(t).UnixMilli()
}

// Ms2Time 将毫秒级 Unix 时间戳转换为 time.Time。
func Ms2Time(ms int64) time.Time {
	// 将毫秒分解为秒和纳秒（余毫秒 * 1e6）以保留毫秒精度
	return ToUTC(time.Unix(ms/SecMs, ms%SecMs*1e6))
}

// Time2Sec 将 time.Time 转换为秒级 Unix 时间戳。
func Time2Sec(t time.Time) int64 {
	return ToUTC(t).Unix()
}

// Sec2Time 将秒级 Unix 时间戳转换为 time.Time。
func Sec2Time(t int64) time.Time {
	return ToUTC(time.Unix(t, 0))
}

// Ms2Sec 将毫秒时间戳转换为秒时间戳（截断，不四舍五入）。
func Ms2Sec(t int64) int64 {
	return t / SecMs
}

// Sec2Ms 将秒时间戳转换为毫秒时间戳。
func Sec2Ms(t int64) int64 {
	return t * SecMs
}

// Min2Ms 将分钟数转换为毫秒数。
func Min2Ms(t int32) int64 {
	return int64(t) * MinMs
}

// Hour2Ms 将小时数转换为毫秒数。
func Hour2Ms(t int32) int64 {
	return int64(t) * HourMs
}

// Day2Ms 将天数转换为毫秒数。
func Day2Ms(t int32) int64 {
	return int64(t) * DayMs
}

// FormatMs 将毫秒时间戳格式化为 "2006-01-02 15:04:05" 格式的字符串。
func FormatMs(ms int64) string {
	return Ms2Time(ms).Format(DefaultTimeFormat)
}

// TodayStartTs 获取今日零点（00:00:00.000）的毫秒时间戳。
//
// 基于当前逻辑时间（Now）计算，而非系统时间，
// 保证在启用时间偏移的测试场景下行为一致。
func TodayStartTs() int64 {
	now := Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return todayStart.UnixMilli()
}

// NextDayTs 获取指定毫秒时间戳所在日期的次日零点时间戳。
//
// 用于计算"明天几点"类的业务逻辑，如每日任务重置时间。
func NextDayTs(nowMs int64) int64 {
	now := Ms2Time(nowMs)
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	nextDay := nowDay.AddDate(0, 0, 1)
	return nextDay.UnixMilli()
}
