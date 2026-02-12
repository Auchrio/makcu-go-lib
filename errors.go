package Macku

import "errors"

// Sentinel errors for type checking with errors.Is().
var (
	ErrConnection = errors.New("macku: connection error")
	ErrCommand    = errors.New("macku: command error")
	ErrTimeout    = errors.New("macku: timeout")
	ErrResponse   = errors.New("macku: response error")
)

// MakcuError wraps a sentinel error with a descriptive message.
type MakcuError struct {
	Base    error
	Message string
}

func (e *MakcuError) Error() string {
	return e.Message
}

func (e *MakcuError) Unwrap() error {
	return e.Base
}

// NewConnectionError creates a connection error.
func NewConnectionError(msg string) error {
	return &MakcuError{Base: ErrConnection, Message: msg}
}

// NewCommandError creates a command error.
func NewCommandError(msg string) error {
	return &MakcuError{Base: ErrCommand, Message: msg}
}

// NewTimeoutError creates a timeout error.
func NewTimeoutError(msg string) error {
	return &MakcuError{Base: ErrTimeout, Message: msg}
}

// NewResponseError creates a response error.
func NewResponseError(msg string) error {
	return &MakcuError{Base: ErrResponse, Message: msg}
}
