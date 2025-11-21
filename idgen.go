package utility

import (
	"strconv"
	"sync"
	"time"

	"github.com/wildmap/utility/xtime"
)

/*
 * uint64 ID Generator
 * Encoding format:
 * 41-bit timestamp (second precision) + 22-bit auto-increment sequence number
 * This design allows for unique ID generation across distributed systems
 */

const (
	// timeBits defines the number of bits allocated for timestamp storage (41 bits)
	timeBits uint64 = 41

	// seqBits defines the number of bits allocated for sequence number (22 bits)
	seqBits uint64 = 22

	// maxTime is the maximum timestamp value that can be stored (2^41 - 1)
	maxTime uint64 = 1<<timeBits - 1

	// maxSeq is the maximum sequence number value (2^22 - 1, about 4 million)
	maxSeq uint64 = 1<<seqBits - 1

	// timeShift defines the left shift amount for timestamp (22 bits)
	timeShift uint64 = seqBits

	// seqShift defines the left shift amount for sequence number (0 bits, no shift)
	seqShift uint64 = 0

	// maxId is the maximum ID value that can be generated (2^63 - 1)
	maxId uint64 = (1 << (timeBits + seqBits)) - 1
)

// ID is a type alias for int64 representing a unique identifier
// It provides methods for conversion and extraction of timestamp/sequence components
type ID int64

// String converts the ID to its string representation
func (i ID) String() string { return strconv.FormatUint(uint64(i), 10) }

// Uint64 converts the ID to an unsigned 64-bit integer
func (i ID) Uint64() uint64 { return uint64(i) }

// Int64 converts the ID to a signed 64-bit integer
func (i ID) Int64() int64 { return int64(i) }

// Float64 converts the ID to a 64-bit floating point number
func (i ID) Float64() float64 {
	return float64(i)
}

// Bytes converts the ID to a byte slice of its string representation
func (i ID) Bytes() []byte { return []byte(i.String()) }

// Time extracts and returns the timestamp component of the ID as a time.Time object
// The timestamp is stored in the upper 41 bits of the ID
func (i ID) Time() time.Time {
	t := uint64(i) >> timeShift & maxTime
	return time.Unix(int64(t), 0)
}

// Seq extracts and returns the sequence number component of the ID
// The sequence number is stored in the lower 22 bits of the ID
func (i ID) Seq() uint64 {
	return uint64(i) >> seqShift & maxSeq
}

// generator is the core structure for ID generation
// It maintains the sequence counter and last timestamp to ensure uniqueness
type generator struct {
	mu        sync.Mutex // Mutex to ensure thread-safe ID generation
	sequence  uint64     // Current sequence number, increments with each ID
	lastStamp uint64     // Last timestamp used, in seconds since Unix epoch
}

// newGenerator creates and initializes a new ID generator instance
// It sets the initial sequence to 1 and records the current timestamp
func newGenerator() *generator {
	s := &generator{
		sequence: 1,
	}
	s.lastStamp = s.currentMillis()
	return s
}

// currentMillis returns the current Unix timestamp in seconds
func (s *generator) currentMillis() uint64 {
	return uint64(xtime.NowSecTs())
}

// NextId generates the next unique ID
// It handles sequence overflow by advancing the timestamp and resetting the sequence
// Thread-safe through mutex locking
func (s *generator) NextId() ID {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for timestamp overflow (exceeds 41-bit capacity)
	if s.lastStamp > maxTime {
		panic("timestamp overflow")
	}

	// Check for sequence overflow (exceeds 22-bit capacity)
	if s.sequence > maxSeq {
		// Wait for time to advance to next second
		for s.lastStamp > s.currentMillis() {
			time.Sleep(time.Millisecond)
		}
		// Advance timestamp and reset sequence
		s.lastStamp++
		s.sequence = 1
	} else {
		// Simply increment sequence number
		s.sequence++
	}

	// Combine timestamp and sequence into final ID
	// Format: [41-bit timestamp][22-bit sequence]
	return ID(((s.lastStamp << timeShift) | (s.sequence << seqShift)) & maxId)
}

var (
	// idgen is the global singleton ID generator instance
	idgen = newGenerator()
)

// NewID generates a new unique ID using the global generator
// This is the primary function for creating new IDs
func NewID() ID {
	return idgen.NextId()
}

// ParseID converts a uint64 value to an ID type
// Useful for deserializing IDs from storage or network
func ParseID(id uint64) ID {
	return ID(id)
}

// ParseString converts a string representation to an ID
func ParseString(id string) (ID, error) {
	v, err := strconv.ParseUint(id, 10, 64)
	return ID(v), err
}
