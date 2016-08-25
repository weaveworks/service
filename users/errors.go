package users

import (
	"errors"
	"fmt"
)

// MalformedInputError is an error on malformed input
type MalformedInputError error

// ValidationError is an error of data validation
type ValidationError error

// ValidationErrorf creates a new validation error
func ValidationErrorf(format string, args ...interface{}) ValidationError {
	return ValidationError(fmt.Errorf(format, args...))
}

// These are specific instances of errors the users application deals with.
var (
	ErrForbidden                  = errors.New("Forbidden found")
	ErrNotFound                   = errors.New("Not found")
	ErrEmailIsTaken               = ValidationErrorf("Email is already taken")
	ErrInvalidAuthenticationData  = errors.New("Invalid authentication data")
	ErrOrgExternalIDIsTaken       = ValidationErrorf("ID is already taken")
	ErrOrgExternalIDCannotBeBlank = ValidationErrorf("ID cannot be blank")
	ErrOrgExternalIDFormat        = ValidationErrorf("ID can only contain letters, numbers, hyphen, and underscore")
	ErrOrgNameCannotBeBlank       = ValidationErrorf("Name cannot be blank")
)
