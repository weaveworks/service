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

// AlreadyAttachedError is when an oauth login is already attached to some other account
type AlreadyAttachedError struct {
	ID    string
	Email string
}

func (err AlreadyAttachedError) Error() string {
	return fmt.Sprintf("Login is already attached to %q", err.Email)
}

// Metadata implements WithMetadata
func (err AlreadyAttachedError) Metadata() map[string]interface{} {
	return map[string]interface{}{"email": err.Email}
}

// WithMetadata is the interface errors should implement if they want to
// include other information when rendered via the API.
type WithMetadata interface {
	Metadata() map[string]interface{}
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
