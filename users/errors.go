package users

import (
	"errors"
	"fmt"
)

// MalformedInputError is an error on malformed input
type MalformedInputError struct {
	err error
}

// NewMalformedInputError wraps an error to denote invalid input.
func NewMalformedInputError(err error) error {
	return &MalformedInputError{err}
}

// Error returns the text of the wrapped error.
func (e *MalformedInputError) Error() string {
	return e.err.Error()
}

// ValidationError is an error of data validation
type ValidationError struct {
	s string
}

// Error returns the text.
func (e *ValidationError) Error() string {
	return e.s
}

// ValidationErrorf creates a new validation error
func ValidationErrorf(format string, args ...interface{}) error {
	return &ValidationError{s: fmt.Sprintf(format, args...)}
}

// AlreadyAttachedError is when an oauth login is already attached to some other account
type AlreadyAttachedError struct {
	ID    string
	Email string
}

func (err *AlreadyAttachedError) Error() string {
	return fmt.Sprintf("Login is already attached to %q", err.Email)
}

// Metadata implements WithMetadata
func (err *AlreadyAttachedError) Metadata() map[string]interface{} {
	return map[string]interface{}{"email": err.Email}
}

// WithMetadata is the interface errors should implement if they want to
// include other information when rendered via the API.
type WithMetadata interface {
	Metadata() map[string]interface{}
}

// These are specific instances of errors the users application deals with.
var (
	ErrForbidden                  = errors.New("forbidden")
	ErrNotFound                   = errors.New("not found")
	ErrEmailIsTaken               = ValidationErrorf("email is already taken")
	ErrInvalidAuthenticationData  = errors.New("invalid authentication data")
	ErrOrgExternalIDIsTaken       = ValidationErrorf("ID is already taken")
	ErrOrgExternalIDCannotBeBlank = ValidationErrorf("ID cannot be blank")
	ErrOrgExternalIDFormat        = ValidationErrorf("ID can only contain letters, numbers, hyphen, and underscore")
	ErrOrgNameCannotBeBlank       = ValidationErrorf("name cannot be blank")
	ErrOrgPlatformInvalid         = ValidationErrorf("platform is invalid")
	ErrOrgEnvironmentInvalid      = ValidationErrorf("environment is invalid")
	ErrOrgPlatformRequired        = ValidationErrorf("platform is required with environment")
	ErrOrgEnvironmentRequired     = ValidationErrorf("environment is required with platform")
	ErrOrgTrialExpiresInvalid     = ValidationErrorf("trial has no expiration set")
	ErrOrgTokenIsTaken            = errors.New("token already taken")
	ErrLoginNotFound              = errors.New("no login for this user")
	ErrProviderParameters         = errors.New("must pass provider and userID")
	ErrInstanceDataAccessDenied   = errors.New("access to data from this instance is prohibited")
	ErrInstanceDataUploadDenied   = errors.New("uploading new data to this instance is prohibited")
)
