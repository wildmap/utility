package xtime

import (
	"time"
)

// ==================== 时间单位常量（毫秒） ====================

const (
	SecMs  = 1000        // 1秒 = 1000毫秒
	MinMs  = 60 * SecMs  // 1分钟 = 60秒
	HourMs = 60 * MinMs  // 1小时 = 60分钟
	DayMs  = 24 * HourMs // 1天 = 24小时
)

// ==================== 时间单位常量（秒） ====================

const (
	SecondPerMinute = 60           // 每分钟秒数
	SecondPerHour   = 60 * 60      // 每小时秒数
	SecondPerDay    = 24 * 60 * 60 // 每天秒数
)

// ==================== 时间格式常量 ====================

const (
	StdTimeFormat       = "2006-01-02 15:04:05.999" // 标准时间格式（含毫秒）
	StdTimeFormatMinute = "2006-01-02 15:04"        // 标准时间格式（到分钟）
	DefaultTimeFormat   = "2006-01-02 15:04:05"     // 默认时间格式
)

// ==================== 星期常量 ====================

const (
	WeekDayNone      int32 = 0
	WeekDayMonday    int32 = 1
	WeekDayTuesday   int32 = 2
	WeekDayWednesday int32 = 3
	WeekDayThursday  int32 = 4
	WeekDayFriday    int32 = 5
	WeekDaySaturday  int32 = 6
	WeekDaySunday    int32 = 7
)

// ==================== 全局配置变量 ====================

var (
	// LocalTime 是否使用本地时间（false表示使用UTC）
	LocalTime bool = false

	// TimeZoneName 时区名称
	TimeZoneName = ""

	// TimeZoneOffset 时区偏移量（秒）
	TimeZoneOffset = 0
)

// ==================== 初始化 ====================

func init() {
	TimeZoneName, TimeZoneOffset = time.Now().Local().Zone()
}

// ==================== 时区配置函数 ====================

// SetLocalTime 设置是否使用本地时间
func SetLocalTime(localtime bool) {
	LocalTime = localtime
}

// ToUTC 转换为UTC时间或保持本地时间
func ToUTC(t time.Time) time.Time {
	if LocalTime {
		return t
	}
	return t.UTC()
}

// Parse 解析时间字符串
func Parse(timeStr string) (time.Time, error) {
	if LocalTime {
		return time.ParseInLocation(DefaultTimeFormat, timeStr, time.Local)
	}
	return time.Parse(DefaultTimeFormat, timeStr)
}