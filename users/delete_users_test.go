package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_DeleteUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, err := storage.CreateUser("joe@weave.works")
	require.NoError(t, err)
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	assert.NotEqual(t, "", user.Organization.ID)

	fran, err := storage.CreateUser("fran@weave.works")
	require.NoError(t, err)
	fran, err = storage.InviteUser(fran.Email, user.Organization.Name)
	require.NoError(t, err)
	assert.Equal(t, user.Organization.ID, fran.Organization.ID)

	cookie, err := sessions.Cookie(user.ID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("DELETE", "/api/users/org/"+user.Organization.Name+"/users/"+fran.Email, nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	_, err = storage.FindUserByEmail("fran@weave.works")
	assert.Equal(t, errNotFound, err)
}
