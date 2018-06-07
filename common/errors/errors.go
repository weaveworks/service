package errors

import "errors"

var (
	// ErrNotFound is a generic not found error.
	ErrNotFound = errors.New("not found")

	// ErrInvalidParameter is a generic invalid parameter error
	ErrInvalidParameter = errors.New("invalid parameter")
)
