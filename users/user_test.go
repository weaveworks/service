package users_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users"
)

func Test_User_CompareToken(t *testing.T) {
	valid := "s3cur1ty"
	h, err := bcrypt.GenerateFromPassword([]byte(valid), bcrypt.MinCost)
	assert.NoError(t, err)
	hashedToken := string(h)
	u := &users.User{
		Token:          hashedToken,
		TokenCreatedAt: time.Now().UTC(),
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
	u.TokenCreatedAt = time.Now().UTC().Add(-73 * time.Hour)
	assert.False(t, u.CompareToken(valid))

	// Works on nil user
	var nilUser *users.User
	assert.False(t, nilUser.CompareToken(""))
}
