package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users/client"
)

func Test_Account_AttachOauthAccount(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)

	remoteEmail := "fran@example.com"
	logins.Register("mock", MockLoginProvider{
		// Different remote email, to prevent auto-matching
		"joe": {ID: "joe", Email: remoteEmail},
	})

	// Hit the endpoint that the oauth login will redirect to (with our session)
	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/logins/mock/attach?code=joe&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, client.AuthCookieName))
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"attach":       true,
		"firstLogin":   true,
		"email":        remoteEmail,
		"munchkinHash": app.MunchkinHash(remoteEmail),
	}, body)
	assert.Len(t, sentEmails, 0)

	// Should have logged us in as the same user.
	found, err := database.FindUserByLogin(context.Background(), "mock", "joe")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)

	// User should have an login set
	logins, err := database.ListLoginsForUserIDs(context.Background(), found.ID)
	require.NoError(t, err)
	if assert.Len(t, logins, 1) {
		assert.Equal(t, user.ID, logins[0].UserID)
		assert.Equal(t, "mock", logins[0].Provider)
		assert.Equal(t, "joe", logins[0].ProviderID)
	}
}

func Test_Account_AttachOauthAccount_AlreadyAttachedToAnotherAccount(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	fran := getUser(t)
	require.NoError(t, database.AddLoginToUser(context.Background(), fran.ID, "mock", "fran", nil))
	fran, err := database.FindUserByID(context.Background(), fran.ID)
	require.NoError(t, err)
	franLogins, err := database.ListLoginsForUserIDs(context.Background(), fran.ID)
	require.NoError(t, err)
	assert.Len(t, franLogins, 1)

	// Should be associated to another user

	logins.Register("mock", MockLoginProvider{
		// Different remote email, to prevent auto-matching
		"fran": {ID: "fran", Email: fran.Email},
	})

	// Hit the endpoint that the oauth login will redirect to (with our session), should fail initially
	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/logins/mock/attach?code=fran&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"errors": []interface{}{
			map[string]interface{}{
				"message": fmt.Sprintf("Login is already attached to %q", fran.Email),
				"email":   fran.Email,
			},
		},
	}, body)
	assert.Len(t, sentEmails, 0)

	// Force the attach
	w = httptest.NewRecorder()
	r = requestAs(t, user, "GET", "/api/users/logins/mock/attach?code=fran&state=state&force=true", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, client.AuthCookieName))
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"attach":       true,
		"firstLogin":   true,
		"email":        fran.Email,
		"munchkinHash": app.MunchkinHash(fran.Email),
	}, body)
	assert.Len(t, sentEmails, 0)

	// Lookup by the login should point at the new user
	found, err := database.FindUserByLogin(context.Background(), "mock", "fran")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)

	// User should have the login set
	foundLogins, err := database.ListLoginsForUserIDs(context.Background(), found.ID)
	require.NoError(t, err)
	if assert.Len(t, foundLogins, 1) {
		assert.Equal(t, user.ID, foundLogins[0].UserID)
		assert.Equal(t, "mock", foundLogins[0].Provider)
		assert.Equal(t, "fran", foundLogins[0].ProviderID)
	}

	// Old user should not be associated anymore
	foundLogins, err = database.ListLoginsForUserIDs(context.Background(), fran.ID)
	require.NoError(t, err)
	assert.Len(t, foundLogins, 0)
}

func Test_Account_AttachOauthAccount_AlreadyAttachedToSameAccount(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)

	// Should be associated to same user
	assert.NoError(t, database.AddLoginToUser(context.Background(), user.ID, "mock", "joe", nil))
	user, err := database.FindUserByID(context.Background(), user.ID)
	assert.NoError(t, err)
	userLogins, err := database.ListLoginsForUserIDs(context.Background(), user.ID)
	assert.NoError(t, err)
	assert.Len(t, userLogins, 1)

	remoteEmail := "fran@example.com"
	logins.Register("mock", MockLoginProvider{
		// Different remote email, to prevent auto-matching
		"joe": {ID: "joe", Email: remoteEmail},
	})

	// Hit the endpoint that the oauth login will redirect to (with our session)
	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/logins/mock/attach?code=joe&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, client.AuthCookieName))
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"attach":       true,
		"firstLogin":   true,
		"email":        remoteEmail,
		"munchkinHash": app.MunchkinHash(remoteEmail),
	}, body)
	assert.Len(t, sentEmails, 0)

	// Lookup by the login should point at the same user
	found, err := database.FindUserByLogin(context.Background(), "mock", "joe")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)

	// User should have the login set
	logins, err := database.ListLoginsForUserIDs(context.Background(), found.ID)
	assert.NoError(t, err)
	if assert.Len(t, logins, 1) {
		assert.Equal(t, user.ID, logins[0].UserID)
		assert.Equal(t, "mock", logins[0].Provider)
		assert.Equal(t, "joe", logins[0].ProviderID)
	}
}

func Test_Account_ListAttachedLoginProviders(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)

	logins.Register("mock", MockLoginProvider{
		// Different remote email, to prevent auto-matching
		"joe": {ID: "joe", Email: user.Email},
	})

	// Listing when none attached
	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/attached_logins", nil)
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
		assert.NoError(t, database.AddLoginToUser(context.Background(), user.ID, "mock", "joe", json.RawMessage(`"joe"`)))
		logins, err := database.ListLoginsForUserIDs(context.Background(), user.ID)
		assert.NoError(t, err)
		assert.Len(t, logins, 1)

		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/attached_logins", nil)
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

	user := getUser(t)

	mockLoginProvider := MockLoginProvider{"joe": {ID: "joe", Email: user.Email}}
	logins.Register("mock", mockLoginProvider)

	assert.NoError(t, database.AddLoginToUser(context.Background(), user.ID, "mock", "joe", json.RawMessage(`"joe"`)))

	// Hit the endpoint that the oauth login will redirect to (with our session)
	w := httptest.NewRecorder()
	r := requestAs(t, user, "POST", "/api/users/logins/mock/detach", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Len(t, w.Body.Bytes(), 0)

	// User should have no more logins set
	userLogins, err := database.ListLoginsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Len(t, userLogins, 0)

	// Provider session should have been revoked.
	assert.Len(t, mockLoginProvider, 0)
}
