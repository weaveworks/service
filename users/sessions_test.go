package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Sessions_EncodeDecode(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	assert.NoError(t, err)

	encoded, err := sessions.Encode(user.ID)
	assert.NoError(t, err)

	found, err := sessions.Decode(encoded)
	require.NoError(t, err)

	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, user.Email, found.Email)
}

func Test_Sessions_Get_NoCookie(t *testing.T) {
	setup(t)
	defer cleanup(t)

	r, _ := http.NewRequest("GET", "/", nil)
	user, err := sessions.Get(r)
	assert.Nil(t, user)
	assert.Equal(t, err, ErrInvalidAuthenticationData)
}
