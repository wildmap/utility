package xtime

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"go.uber.org/atomic"
)

// ==================== 时间偏移变量 ====================

var (
	// useOffset 是否使用时间偏移（用于测试环境）
	useOffset = false

	// offset 逻辑时间偏移量
	offset atomic.Duration
)

// ==================== 时间偏移函数 ====================

// SetUseOffset 设置是否使用时间偏移（非生产环境）
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

// ==================== 获取当前时间函数 ====================

// Now 获取当前时间（考虑时间偏移）
func Now() time.Time {
	if useOffset {
		return time.Now().Add(offset.Load())
	}
	return ToUTC(time.Now())
}

// NowUTC 获取当前UTC时间
func NowUTC() time.Time {
	return ToUTC(Now())
}

// TimeUTCNow 获取当前UTC时间（别名）
func TimeUTCNow() time.Time {
	return Now().UTC()
}

// NowSecTs 获取当前秒级时间戳
func NowSecTs() int64 {
	return ToUTC(Now()).Unix()
}

// TimeUTCSecNow 获取当前UTC秒级时间戳（别名）
func TimeUTCSecNow() int64 {
	return Now().UTC().Unix()
}

// NowTs 获取当前毫秒级时间戳
func NowTs() int64 {
	return ToUTC(Now()).UnixNano() / 1e6
}

// NowUsTs 获取当前微秒级时间戳
func NowUsTs() int64 {
	return ToUTC(Now()).UnixNano() / 1e3
}

// ==================== 时间戳转换 ====================

// Time2Ms 将time.Time转换为毫秒时间戳
func Time2Ms(t time.Time) int64 {
	return ToUTC(t).UnixNano() / 1e6
}

// SysTime2Ts 系统时间转换为毫秒时间戳（别名）
func SysTime2Ts(t time.Time) int64 {
	return Time2Ms(t)
}

// Ms2Time 将毫秒时间戳转换为time.Time
func Ms2Time(ms int64) time.Time {
	return ToUTC(time.Unix(ms/SecMs, ms%SecMs*1e6))
}

// TimeUTCSecToTime 将秒级时间戳转换为time.Time
func TimeUTCSecToTime(t int64) time.Time {
	return time.Unix(t, 0).UTC()
}

// ==================== 时间单位转换 ====================

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

// Ms2Day 毫秒时间戳转换为天数（从1970-01-01开始）
func Ms2Day(t int64) int64 {
	if LocalTime {
		t = t + int64(TimeZoneOffset)*SecMs
	}
	return t / DayMs
}

// Ms2Hour 毫秒转小时（浮点数）
func Ms2Hour(dura int64) float32 {
	return float32(dura) / HourMs
}

// ==================== 时间格式化函数 ====================

// FormatTime 将毫秒时间戳格式化为标准时间字符串
func FormatTime(ts int64) string {
	return Ms2Time(ts).Format(DefaultTimeFormat)
}

// NowToFormatBI 格式化为BI数据格式（含毫秒）
func NowToFormatBI(ms int64) string {
	return time.Unix(ms/1e3, ms%1e3*1e6).Format(StdTimeFormat)
}

// NowToFormatMinute 格式化为分钟级格式
func NowToFormatMinute(ms int64) string {
	utcTime := time.Unix(ms/SecMs, (ms%SecMs)*int64(time.Millisecond)).UTC()
	return utcTime.Format(StdTimeFormatMinute)
}

// ParseTimeStrToMs 解析标准时间字符串为毫秒时间戳
// 失败返回-1
func ParseTimeStrToMs(timeStr string) int64 {
	t, err := Parse(timeStr)
	if err != nil {
		return -1
	}
	return t.Unix() * SecMs
}

// ==================== 天相关计算函数 ====================

// TodayStartTs 获取今天零点的毫秒时间戳
func TodayStartTs() int64 {
	now := Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, ToUTC(now).Location())
	return todayStart.UnixNano() / 1e6
}

// DayStartTs 获取指定毫秒时间戳当天零点的时间戳
func DayStartTs(nowMs int64) int64 {
	now := Ms2Time(nowMs)
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return Time2Ms(nowDay)
}

// NextDayTs 获取下一个UTC零点的毫秒时间戳
func NextDayTs(nowMs int64) int64 {
	now := Ms2Time(nowMs)
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, ToUTC(now).Location())
	nextDay := nowDay.AddDate(0, 0, 1)
	return ToUTC(nextDay).UnixNano() / 1e6
}

// BeginningOfTheDay 获取当天开始的秒级时间戳
func BeginningOfTheDay() int64 {
	now := Now().UTC().Unix()
	return now - (now % SecondPerDay)
}

// BeginningOfTheDayNew 获取当天开始的秒级时间戳（新版本）
func BeginningOfTheDayNew() int64 {
	t := ToUTC(Now())
	addTime := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return addTime.Unix()
}

// IsSameDay 判断两个毫秒时间戳是否为同一天
func IsSameDay(ts1, ts2 int64) bool {
	return DayStartTs(ts1) == DayStartTs(ts2)
}

// IsTheSameDay 判断秒级时间戳是否为今天
func IsTheSameDay(timestamp int64) bool {
	if timestamp == 0 {
		return false
	}

	now := TimeUTCNow()
	checkTime := TimeUTCSecToTime(timestamp)

	nowYear, nowMon, nowDay := now.Date()
	checkYear, checkMon, checkDay := checkTime.Date()

	return nowYear == checkYear && nowMon == checkMon && nowDay == checkDay
}

// DiffDay 计算两个秒级时间戳之间相差的天数
func DiffDay(t1 int64, t2 int64) int32 {
	if t1 == t2 {
		return 0
	}
	if t1 > t2 {
		t1, t2 = t2, t1
	}

	diffDays := 0
	secDiff := t2 - t1

	if secDiff > SecondPerDay {
		tmpDays := int(secDiff / SecondPerDay)
		t1 += int64(tmpDays) * SecondPerDay
		diffDays += tmpDays
	}

	st := time.Unix(t1, 0)
	et := time.Unix(t2, 0)
	dateFormatTpl := "20060102"
	if st.Format(dateFormatTpl) != et.Format(dateFormatTpl) {
		diffDays++
	}

	return int32(diffDays)
}

// ==================== 周相关计算函数 ====================

// GetMonDayZero 获取本周一零点的毫秒时间戳
func GetMonDayZero() int64 {
	now := Now()
	change := int(time.Monday - now.Weekday())
	if change > 0 {
		change = -6
	}
	weekStartDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, ToUTC(now).Location()).AddDate(0, 0, change)
	return weekStartDate.UnixNano() / 1e6
}

// GetMonDayZeroTs 获取本周一零点的秒级时间戳
func GetMonDayZeroTs() int64 {
	return GetMonDayZero() / SecMs
}

// FirstDayOfWeekTs 获取指定时间所在周的周一零点时间戳
func FirstDayOfWeekTs(nowMs int64) int64 {
	now := time.Unix(0, nowMs*1e6)
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.UTC().Location())

	_offset := int(time.Monday - now.Weekday())
	if _offset > 0 {
		_offset = -6
	}
	nextDay := nowDay.AddDate(0, 0, _offset)
	return nextDay.UTC().UnixNano() / 1e6
}

// NextFirstDayOfWeekTs 获取下周一零点的时间戳
func NextFirstDayOfWeekTs(nowMs int64) int64 {
	now := time.Unix(0, nowMs*1e6)
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.UTC().Location())

	_offset := int(time.Monday - now.Weekday())
	if _offset > 0 {
		_offset = -6
	}
	nextDay := nowDay.AddDate(0, 0, _offset+7)
	return nextDay.UTC().UnixNano() / 1e6
}

// WeekMs 获取周日零点的时间戳
func WeekMs(nowMs int64) int64 {
	t := Ms2Time(nowMs)
	d := time.Date(t.Year(), t.Month(), t.Day()-int(t.Weekday()), 0, 0, 0, 0, t.Location())
	return Time2Ms(d)
}

// WeekdayTs 获取指定时间所在周的某个工作日零点时间戳
func WeekdayTs(nowMs int64, wd time.Weekday) int64 {
	now := time.Unix(0, nowMs*1e6)
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.UTC().Location())

	_offset := int(wd - now.Weekday())
	if _offset < 0 {
		_offset += 7
	}
	nextDay := nowDay.AddDate(0, 0, _offset)
	return nextDay.UTC().UnixNano() / 1e6
}

// NextWeekdayTs 获取指定时间之后的某个工作日零点时间戳
func NextWeekdayTs(nowMs int64, wd time.Weekday) int64 {
	now := Ms2Time(nowMs)
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.UTC().Location())

	_offset := int(wd - now.Weekday())
	if _offset <= 0 {
		_offset += 7
	}

	nextDay := nowDay.AddDate(0, 0, _offset)
	return nextDay.UTC().UnixNano() / 1e6
}

// ==================== 小时相关计算函数 ====================

// NextHourDuration 计算到达下一个指定小时的毫秒数
func NextHourDuration(ms int64, hour int) int64 {
	t := Ms2Time(ms)
	next := time.Date(t.Year(), t.Month(), t.Day(), hour, 0, 0, 0, t.Location())
	if !t.Before(next) {
		next = next.Add(24 * time.Hour)
	}
	return int64(next.Sub(t) / time.Millisecond)
}

// DiffHours 计算两个时间戳之间相差的小时数（向下取整）
func DiffHours(t1, t2 int64) int32 {
	if t2 < t1 {
		t1, t2 = t2, t1
	}

	diffMs := t2 - t1
	diffHour := float64(diffMs) / HourMs

	return int32(math.Floor(diffHour))
}

// SetNextDayTime 返回下一天指定时间点的时间戳
// targetTime格式: "20:00:00"
func SetNextDayTime(targetTime string) int64 {
	hhddssArr := strings.Split(targetTime, ":")
	if len(hhddssArr) != 3 {
		slog.Error(fmt.Sprintf("SetNextDayTime targetTime format error: %s", targetTime))
		return 0
	}

	nextDay := Now().AddDate(0, 0, 1)
	hh, _ := strconv.Atoi(hhddssArr[0])
	mm, _ := strconv.Atoi(hhddssArr[1])
	dd, _ := strconv.Atoi(hhddssArr[2])
	resultDay := time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), hh, mm, dd, 0, nextDay.UTC().Location())

	slog.Debug(fmt.Sprintf("SetNextDayTime targetTime: %s, nextDay: %v, resultDay:%v", targetTime, nextDay, resultDay))
	return resultDay.UTC().UnixNano() / 1e6
}
