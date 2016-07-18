package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Account_AttachOauthAccount(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getApprovedUser(t)

	remoteEmail := "fran@example.com"
	logins.Register("mock", MockLoginProvider{
		// Different remote email, to prevent auto-matching
		"joe": {ID: "joe", Email: remoteEmail},
	})

	// Hit the endpoint that the oauth login will redirect to (with our session)
	w := httptest.NewRecorder()
	r, _ := requestAs(t, user, "GET", "/api/users/logins/mock/attach?code=joe&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, cookieName))
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"attach":     true,
		"firstLogin": true,
	}, body)
	assert.Len(t, sentEmails, 0)

	// Should have logged us in as the same user.
	found, err := storage.FindUserByLogin("mock", "joe")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)

	// User should have an login set
	if assert.Len(t, found.Logins, 1) {
		assert.Equal(t, user.ID, found.Logins[0].UserID)
		assert.Equal(t, "mock", found.Logins[0].Provider)
		assert.Equal(t, "joe", found.Logins[0].ProviderID)
	}
}

func Test_Account_AttachOauthAccount_AlreadyAttachedToAnotherAccount(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getApprovedUser(t)
	fran := getApprovedUser(t)
	require.NoError(t, storage.AddLoginToUser(fran.ID, "mock", "fran", nil))
	fran, err := storage.FindUserByID(fran.ID)
	require.NoError(t, err)
	assert.Len(t, fran.Logins, 1)

	// Should be associated to another user

	logins.Register("mock", MockLoginProvider{
		// Different remote email, to prevent auto-matching
		"fran": {ID: "fran", Email: fran.Email},
	})

	// Hit the endpoint that the oauth login will redirect to (with our session)
	w := httptest.NewRecorder()
	r, _ := requestAs(t, user, "GET", "/api/users/logins/mock/attach?code=fran&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, cookieName))
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"attach":     true,
		"firstLogin": true,
	}, body)
	assert.Len(t, sentEmails, 0)

	// Lookup by the login should point at the new user
	found, err := storage.FindUserByLogin("mock", "fran")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)

	// User should have the login set
	if assert.Len(t, found.Logins, 1) {
		assert.Equal(t, user.ID, found.Logins[0].UserID)
		assert.Equal(t, "mock", found.Logins[0].Provider)
		assert.Equal(t, "fran", found.Logins[0].ProviderID)
	}

	// Old user should not be assocaited anymore
	fran, err = storage.FindUserByID(fran.ID)
	require.NoError(t, err)
	assert.Len(t, fran.Logins, 0)
}

func Test_Account_AttachOauthAccount_AlreadyAttachedToSameAccount(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getApprovedUser(t)

	// Should be associated to same user
	assert.NoError(t, storage.AddLoginToUser(user.ID, "mock", "joe", nil))
	user, err := storage.FindUserByID(user.ID)
	assert.NoError(t, err)
	assert.Len(t, user.Logins, 1)

	remoteEmail := "fran@example.com"
	logins.Register("mock", MockLoginProvider{
		// Different remote email, to prevent auto-matching
		"joe": {ID: "joe", Email: remoteEmail},
	})

	// Hit the endpoint that the oauth login will redirect to (with our session)
	w := httptest.NewRecorder()
	r, _ := requestAs(t, user, "GET", "/api/users/logins/mock/attach?code=joe&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, cookieName))
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"attach":     true,
		"firstLogin": true,
	}, body)
	assert.Len(t, sentEmails, 0)

	// Lookup by the login should point at the same user
	found, err := storage.FindUserByLogin("mock", "joe")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)

	// User should have the login set
	if assert.Len(t, found.Logins, 1) {
		assert.Equal(t, user.ID, found.Logins[0].UserID)
		assert.Equal(t, "mock", found.Logins[0].Provider)
		assert.Equal(t, "joe", found.Logins[0].ProviderID)
	}
}

func Test_Account_ListAttachedLoginProviders(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getApprovedUser(t)

	logins.Register("mock", MockLoginProvider{
		// Different remote email, to prevent auto-matching
		"joe": {ID: "joe", Email: user.Email},
	})

	// Listing when none attached
	{
		w := httptest.NewRecorder()
		r, _ := requestAs(t, user, "GET", "/api/users/attached_logins", nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		var body struct {
			Logins []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				LoginID  string `json:"loginID,omitempty"`
				Username string `json:"username,omitempty"`
			} `json:"logins"`
		}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Len(t, body.Logins, 0)
	}

	// Listing when one attached
	{
		assert.NoError(t, storage.AddLoginToUser(user.ID, "mock", "joe", json.RawMessage(`"joe"`)))
		user, err := storage.FindUserByID(user.ID)
		assert.NoError(t, err)
		assert.Len(t, user.Logins, 1)

		w := httptest.NewRecorder()
		r, _ := requestAs(t, user, "GET", "/api/users/attached_logins", nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		var body struct {
			Logins []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				LoginID  string `json:"loginID,omitempty"`
				Username string `json:"username,omitempty"`
			} `json:"logins"`
		}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Len(t, body.Logins, 1)
		assert.Equal(t, "mock", body.Logins[0].ID)
		assert.Equal(t, "mock", body.Logins[0].Name)
		assert.Equal(t, "joe", body.Logins[0].LoginID)
		assert.Equal(t, user.Email, body.Logins[0].Username)
	}
}

func Test_Account_DetachOauthAccount(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getApprovedUser(t)

	logins.Register("mock", MockLoginProvider{
		"joe": {ID: "joe", Email: user.Email},
	})

	assert.NoError(t, storage.AddLoginToUser(user.ID, "mock", "joe", nil))

	// Hit the endpoint that the oauth login will redirect to (with our session)
	w := httptest.NewRecorder()
	r, _ := requestAs(t, user, "POST", "/api/users/logins/mock/detach", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// User should have no more logins set
	user, err := storage.FindUserByID(user.ID)
	require.NoError(t, err)
	assert.Len(t, user.Logins, 0)
}
