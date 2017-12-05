package users_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/users"
)

func TestErrors_typeMatching(t *testing.T) {
	malformed := users.NewMalformedInputError(errors.New("boom"))
	switch malformed.(type) {
	case *users.ValidationError:
		assert.Fail(t, "MalformedInputError identified as ValidationError")
	}
	validation := users.ValidationErrorf("invalid")
	switch validation.(type) {
	case *users.MalformedInputError:
		assert.Fail(t, "ValidationError identified as MalformedInputError")
	}
}

func TestErrors_comparison(t *testing.T) {
	v0 := users.ValidationErrorf("same")
	v1 := users.ValidationErrorf("same")
	v2 := v0
	assert.False(t, v0 == v1)
	assert.True(t, v0 == v2)

	err := errors.New("boo")
	m0 := users.NewMalformedInputError(err)
	m1 := users.NewMalformedInputError(err)
	assert.False(t, m0 == m1)
}
