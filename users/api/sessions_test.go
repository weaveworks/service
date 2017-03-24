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

	user := getUser(t)

	encoded, err := sessionStore.Encode(user.ID)
	require.NoError(t, err)

	foundSession, err := sessionStore.Decode(encoded)
	require.NoError(t, err)

	assert.Equal(t, user.ID, foundSession.UserID)
}

func Test_Sessions_Get_NoCookie(t *testing.T) {
	setup(t)
	defer cleanup(t)

	r, _ := http.NewRequest("GET", "/", nil)
	session, err := sessionStore.Get(r)
	assert.IsType(t, users.NewInvalidAuthenticationDataError(nil), err)
	assert.Equal(t, "", session.UserID)
}
