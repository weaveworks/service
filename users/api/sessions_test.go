package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users"
)

func Test_Sessions_EncodeDecode(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getApprovedUser(t)

	encoded, err := sessionStore.Encode(user.ID)
	require.NoError(t, err)

	foundID, err := sessionStore.Decode(encoded)
	require.NoError(t, err)

	assert.Equal(t, user.ID, foundID)
}

func Test_Sessions_Get_NoCookie(t *testing.T) {
	setup(t)
	defer cleanup(t)

	r, _ := http.NewRequest("GET", "/", nil)
	userID, err := sessionStore.Get(r)
	assert.Equal(t, users.ErrInvalidAuthenticationData, err)
	assert.Equal(t, "", userID)
}
