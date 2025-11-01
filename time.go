package utility

import (
	"time"

	"go.uber.org/atomic"
)

const (
	SecMs  = 1000
	MinMs  = 60 * SecMs
	HourMs = 60 * MinMs
	DayMs  = 24 * HourMs
)

var (
	// useOffset determines whether to apply time offset for logical time adjustments
	// This value must be set during server startup and should not be changed afterward
	// Used for testing or simulation purposes in non-production environments
	useOffset = false

	// LocalTime determines whether to use local time zone or UTC
	// When false, all times are converted to UTC
	LocalTime = false
)

// offset stores the logical time offset value
// Used to simulate time travel or adjust server time for testing purposes
// Thread-safe atomic variable to prevent race conditions
var offset atomic.Duration

// SetUseOffset enables or disables the use of time offset
// Should only be called in non-production mode during initialization
func SetUseOffset(use bool) {
	useOffset = use
}

// AddOffset adds a duration to the current time offset
// Allows incrementally adjusting the logical time
// Thread-safe operation using atomic storage
func AddOffset(dur time.Duration) {
	offset.Store(offset.Load() + dur)
}

// ClearOffset resets the time offset to zero
// Returns the logical time to match actual system time
func ClearOffset() {
	offset.Store(0)
}

// GetOffset returns the current time offset value
// Useful for debugging or displaying the current time adjustment
func GetOffset() time.Duration {
	return offset.Load()
}

// SetLocalTime configures whether to use local time zone or UTC
// When set to true, times are kept in local timezone
// When set to false, times are converted to UTC
func SetLocalTime(localtime bool) {
	LocalTime = localtime
}

// ToUTC converts a time value to UTC if LocalTime is false
// If LocalTime is true, returns the time unchanged
// Ensures consistent time zone handling across the application
func ToUTC(t time.Time) time.Time {
	if LocalTime {
		return t
	} else {
		return t.UTC()
	}
}

// Now returns the current time with optional offset applied
// If useOffset is enabled, adds the configured offset to current time
// Returns UTC time unless LocalTime is enabled
func Now() time.Time {
	if useOffset {
		return time.Now().Add(offset.Load())
	}
	return ToUTC(time.Now())
}

// NowUTC returns the current time in UTC timezone
// Always returns UTC regardless of LocalTime setting
func NowUTC() time.Time {
	return ToUTC(Now())
}

// NowSecTs returns the current Unix timestamp in seconds
// The timestamp represents seconds elapsed since January 1, 1970 UTC
func NowSecTs() int64 {
	return ToUTC(Now()).UnixNano() / 1e9
}

// NowTs returns the current Unix timestamp in milliseconds
// The timestamp represents milliseconds elapsed since January 1, 1970 UTC
func NowTs() int64 {
	return ToUTC(Now()).UnixNano() / 1e6
}

// NowUsTs returns the current Unix timestamp in microseconds
// The timestamp represents microseconds elapsed since January 1, 1970 UTC
func NowUsTs() int64 {
	return ToUTC(Now()).UnixNano() / 1e3
}

// Sec2Ms converts a timestamp from seconds to milliseconds
// Multiplies the input by 1000 to perform the conversion
func Sec2Ms(t int64) int64 {
	return t * 1000
}
