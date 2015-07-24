package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Sessions_EncodeDecode(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	assert.NoError(t, err)

	encoded, err := sessions.Encode(user.ID)
	assert.NoError(t, err)

	found, err := sessions.Decode(encoded)
	assert.NoError(t, err)

	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, user.Email, found.Email)
}
