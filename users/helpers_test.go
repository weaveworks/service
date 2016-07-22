package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requestAs makes a request as the given user.
func requestAs(t *testing.T, u *user, method, endpoint string, body io.Reader) *http.Request {
	cookie, err := sessions.Cookie(u.ID, "")
	assert.NoError(t, err)

	r, err := http.NewRequest(method, endpoint, body)
	require.NoError(t, err)

	r.AddCookie(cookie)
	return r
}

// getApprovedUser makes a randomly named, approved user
func getApprovedUser(t *testing.T) *user {
	email := fmt.Sprintf("%d@weave.works", rand.Int63())
	user, err := storage.CreateUser(email)
	require.NoError(t, err)

	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)

	return user
}

// getOrg makes org with a random ExternalID and user for testing
func getOrg(t *testing.T) (*user, *organization) {
	user := getApprovedUser(t)

	externalID, err := storage.GenerateOrganizationExternalID()
	require.NoError(t, err)

	org, err := storage.CreateOrganization(user.ID, externalID, externalID)
	require.NoError(t, err)

	assert.NotEqual(t, "", org.ID)
	assert.NotEqual(t, "", org.ExternalID)
	assert.Equal(t, org.ExternalID, org.Label)

	return user, org
}

type jsonBody map[string]interface{}

func (j jsonBody) Reader(t *testing.T) io.Reader {
	b, err := json.Marshal(j)
	require.NoError(t, err)
	return bytes.NewReader(b)
}
