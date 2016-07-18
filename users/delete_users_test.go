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

	user, org := getOrg(t)

	fran, err := storage.CreateUser("fran@weave.works")
	require.NoError(t, err)
	fran, err = storage.InviteUser(fran.Email, org.Name)
	require.NoError(t, err)
	require.Len(t, fran.Organizations, 1)
	assert.Equal(t, org.ID, fran.Organizations[0].ID)

	w := httptest.NewRecorder()
	r, _ := requestAs(t, user, "DELETE", "/api/users/org/"+org.Name+"/users/"+fran.Email, nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	_, err = storage.FindUserByEmail("fran@weave.works")
	assert.Equal(t, errNotFound, err)
}
