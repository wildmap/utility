package xtime

import (
	"time"

	"go.uber.org/atomic"
)

// 毫秒
const (
	SecMs  = 1000        // 1秒 = 1000毫秒
	MinMs  = 60 * SecMs  // 1分钟 = 60秒
	HourMs = 60 * MinMs  // 1小时 = 60分钟
	DayMs  = 24 * HourMs // 1天 = 24小时
)

const (
	StdTimeFormat       = "2006-01-02 15:04:05.999" // 标准时间格式（含毫秒）
	StdTimeFormatMinute = "2006-01-02 15:04"        // 标准时间格式（到分钟）
	DefaultTimeFormat   = "2006-01-02 15:04:05"     // 默认时间格式
)

var (
	// LocalTime 是否使用本地时间（false表示使用UTC）
	LocalTime bool = false

	// TimeZoneName 时区名称
	TimeZoneName = ""

	// TimeZoneOffset 时区偏移量（秒）
	TimeZoneOffset = 0

	// useOffset 是否使用时间偏移
	useOffset = false

	// offset 逻辑时间偏移量
	offset atomic.Duration
)

func init() {
	TimeZoneName, TimeZoneOffset = time.Now().Local().Zone()
}

// Parse 解析时间字符串
func Parse(timeStr string) (time.Time, error) {
	if LocalTime {
		return time.ParseInLocation(DefaultTimeFormat, timeStr, time.Local)
	}
	return time.Parse(DefaultTimeFormat, timeStr)
}

// SetLocalTime 设置是否使用本地时间
func SetLocalTime(localtime bool) {
	LocalTime = localtime
}

// SetUseOffset 设置是否使用时间偏移
func SetUseOffset(use bool) {
	useOffset = use
}

// SetOffset 设置时间偏移量
func SetOffset(dur time.Duration) {
	offset.Store(dur)
}

// AddOffset 增加时间偏移量
func AddOffset(dur time.Duration) {
	offset.Store(offset.Load() + dur)
}

// ChangeTimeTo 将当前时间调整到目标时间（通过设置偏移量）
func ChangeTimeTo(t time.Time) {
	dur := t.Sub(ToUTC(time.Now()))
	SetOffset(dur)
}

// ClearOffset 清除时间偏移
func ClearOffset() {
	offset.Store(0)
}

// GetOffset 获取当前时间偏移量
func GetOffset() time.Duration {
	return offset.Load()
}

// Now 获取当前时间（考虑时区和时间偏移）
func Now() time.Time {
	now := ToUTC(time.Now())
	if useOffset {
		return now.Add(offset.Load())
	}
	return now
}

// NowSecTs 获取当前秒级时间戳
func NowSecTs() int64 {
	return Now().Unix()
}

// NowTs 获取当前毫秒级时间戳
func NowTs() int64 {
	return Now().UnixMilli()
}

// NowUsTs 获取当前微秒级时间戳
func NowUsTs() int64 {
	return Now().UnixMicro()
}

// ToUTC 转换为UTC时间或保持本地时间
func ToUTC(t time.Time) time.Time {
	if LocalTime {
		return t
	}
	return t.UTC()
}

// Time2Ms 将time.Time转换为毫秒时间戳
func Time2Ms(t time.Time) int64 {
	return ToUTC(t).UnixMilli()
}

// Ms2Time 将毫秒时间戳转换为time.Time
func Ms2Time(ms int64) time.Time {
	return ToUTC(time.Unix(ms/SecMs, ms%SecMs*1e6))
}

// Time2Sec 将time.Time转换为秒时间戳
func Time2Sec(t time.Time) int64 {
	return ToUTC(t).Unix()
}

// Sec2Time 将秒级时间戳转换为time.Time
func Sec2Time(t int64) time.Time {
	return ToUTC(time.Unix(t, 0))
}

// Ms2Sec 毫秒转秒
func Ms2Sec(t int64) int64 {
	return t / SecMs
}

// Sec2Ms 秒转毫秒
func Sec2Ms(t int64) int64 {
	return t * SecMs
}

// Min2Ms 分钟转毫秒
func Min2Ms(t int32) int64 {
	return int64(t) * MinMs
}

// Hour2Ms 小时转毫秒
func Hour2Ms(t int32) int64 {
	return int64(t) * HourMs
}

// Day2Ms 天转毫秒
func Day2Ms(t int32) int64 {
	return int64(t) * DayMs
}

// FormatMs 将毫秒时间戳格式化为标准时间字符串
func FormatMs(ms int64) string {
	return Ms2Time(ms).Format(DefaultTimeFormat)
}

// TodayStartTs 获取今天零点的毫秒时间戳
func TodayStartTs() int64 {
	now := Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return todayStart.UnixMilli()
}

// NextDayTs 获取下一个零点的毫秒时间戳
func NextDayTs(nowMs int64) int64 {
	now := Ms2Time(nowMs)
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	nextDay := nowDay.AddDate(0, 0, 1)
	return nextDay.UnixMilli()
}
