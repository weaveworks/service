package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users"
	storagetest "github.com/weaveworks/service/users/storage/test"
)

// RequestAs makes a request as the given user.
func requestAs(t *testing.T, u *users.User, method, endpoint string, body io.Reader) *http.Request {
	cookie, err := sessionStore.Cookie(u.ID)
	assert.NoError(t, err)

	r, err := http.NewRequest(method, endpoint, body)
	require.NoError(t, err)

	r.AddCookie(cookie)
	return r
}

func getApprovedUser(t *testing.T) *users.User {
	return storagetest.GetApprovedUser(t, db)
}

func createOrgForUser(t *testing.T, u *users.User) *users.Organization {
	return storagetest.CreateOrgForUser(t, db, u)
}

func getOrg(t *testing.T) (*users.User, *users.Organization) {
	return storagetest.GetOrg(t, db)
}

type jsonBody map[string]interface{}

func (j jsonBody) Reader(t *testing.T) io.Reader {
	b, err := json.Marshal(j)
	require.NoError(t, err)
	return bytes.NewReader(b)
}
