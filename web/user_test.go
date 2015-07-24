package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func Test_User_CompareToken(t *testing.T) {
	valid := "s3cur1ty"
	h, err := bcrypt.GenerateFromPassword([]byte(valid), bcrypt.MinCost)
	assert.NoError(t, err)
	hashedToken := string(h)
	u := &User{
		Token:       hashedToken,
		TokenExpiry: time.Now().UTC().Add(1 * time.Hour),
	}

	// Matches tokens
	assert.True(t, u.CompareToken(valid))

	// Fails different tokens
	assert.False(t, u.CompareToken("foobar"))

	// Fails blank tokens
	assert.False(t, u.CompareToken(""))

	// Fails all tokens when both tokens are blank
	u.Token = ""
	assert.False(t, u.CompareToken(""))

	// Fails expired tokens
	u.Token = hashedToken
	u.TokenExpiry = time.Now().UTC().Add(-1 * time.Hour)
	assert.False(t, u.CompareToken(valid))
}
