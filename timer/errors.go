package timer

import (
	"errors"
	"fmt"
)

var (
	// ErrNilTimer indicates that the timer is nil/uninitialized
	ErrNilTimer = errors.New("timer: timer is nil")

	// ErrNilNode indicates that the timer node is nil/uninitialized
	ErrNilNode = errors.New("timer: node is nil")

	// ErrNilCallback indicates that the callback function is nil
	ErrNilCallback = errors.New("timer: callback is nil")

	// ErrInvalidExpire indicates that the expiration duration is invalid (e.g., <= 0)
	ErrInvalidExpire = errors.New("timer: invalid expire duration")

	// ErrNodeStopped indicates that the node has already been stopped
	ErrNodeStopped = errors.New("timer: node already stopped")

	// ErrNodeMoved indicates that the node has been moved to another list
	// This typically happens during timewheel cascade operations
	ErrNodeMoved = errors.New("timer: node has been moved")

	// ErrNoRoot indicates that the node doesn't have an associated timewheel
	// This error occurs when trying to reset a node that wasn't properly initialized
	ErrNoRoot = errors.New("timer: node has no root timewheel")

	// ErrHeadNotFound indicates that no appropriate list head was found
	// This should never happen in normal operation and indicates a logic error
	ErrHeadNotFound = errors.New("timer: appropriate head not found")

	// ErrIndexOutOfRange indicates that an array/slice index is out of valid range
	// This is a safety check to prevent panic from invalid indices
	ErrIndexOutOfRange = errors.New("timer: index out of range")

	// ErrTimeGoBack indicates that time has moved backwards
	// This can happen due to system clock adjustments
	ErrTimeGoBack = errors.New("timer: time has gone backwards")

	// ErrNilContext indicates that the context provided is nil
	ErrNilContext = errors.New("timer: context is nil")
)

// Error represents a timer-specific error with additional context information.
// It wraps the original error with operation name and optional message.
type Error struct {
	Op  string // Name of the operation that failed
	Err error  // The underlying/original error
	Msg string // Additional context or description
}

// Error implements the error interface.
// It formats the error message with operation context and additional information.
func (e *Error) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("timer %s error: %v (%s)", e.Op, e.Err, e.Msg)
	}
	return fmt.Sprintf("timer %s error: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error, enabling error chain unwrapping.
// This allows the use of errors.Is() and errors.As() for error checking.
func (e *Error) Unwrap() error {
	return e.Err
}

// newError creates a new timer error with operation context.
// op: the name of the operation that failed
// err: the underlying error
// msg: additional context message (can be empty)
func newError(op string, err error, msg string) error {
	return &Error{
		Op:  op,
		Err: err,
		Msg: msg,
	}
}
