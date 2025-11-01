package utility

// Flag represents a 64-bit unsigned integer used for bitwise flag operations
// Each bit can represent a different boolean state
type Flag uint64

// Set enables the specified flag(s) by performing a bitwise OR operation
// Multiple flags can be set simultaneously by passing combined flag values
func (flag *Flag) Set(f Flag) {
	*flag |= f
}

// Clean removes the specified flag(s) by calling Exclude
// This is a convenience method that clears the given flags
func (flag *Flag) Clean(f Flag) {
	*flag = flag.Exclude(f)
}

// Include checks if all bits in the expected flag are set in the current flag
// Returns true only when every bit in exp is also set in the current flag
func (flag *Flag) Include(exp Flag) bool {
	return (*flag & exp) == exp
}

// IncludeAny checks if any bit in the expected flag is set in the current flag
// Returns true if at least one bit from exp is set in the current flag
func (flag *Flag) IncludeAny(exp Flag) bool {
	return (*flag & exp) != 0
}

// Exclude returns a new flag value with the specified flags removed
// Uses bitwise AND NOT operation to clear the specified bits
func (flag *Flag) Exclude(s Flag) Flag {
	return *flag &^ s
}

// Equal checks if the current flag is exactly equal to the specified flag
// Returns true only when all bits match between the two flags
func (flag *Flag) Equal(s Flag) bool {
	return *flag == s
}
