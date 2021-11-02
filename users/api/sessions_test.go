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

	impersonatingUserID := "" // this test doesn't involve impersonation
	encoded, err := sessionStore.Encode("google", "1234", user.ID, impersonatingUserID)
	require.NoError(t, err)

	foundSession, err := sessionStore.Decode(encoded)
	require.NoError(t, err)

	assert.Equal(t, user.ID, foundSession.UserID)
	assert.Equal(t, "google", foundSession.Provider)
	assert.Equal(t, "1234", foundSession.LoginID)
}

func Test_Sessions_Get_NoCookie(t *testing.T) {
	setup(t)
	defer cleanup(t)

	r, _ := http.NewRequest("GET", "/", nil)
	session, err := sessionStore.Get(r)
	assert.Equal(t, users.ErrInvalidAuthenticationData, err)
	assert.Equal(t, "", session.UserID)
}
