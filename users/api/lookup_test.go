package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func Test_Lookup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/private/api/users/lookup/"+org.ExternalID, nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"organizationID": org.ID, "userID": user.ID}, body)
}

func Test_Lookup_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	cookie, err := sessionStore.Cookie("foouser")
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/lookup/fooorg", nil)
	r.AddCookie(cookie)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func Test_PublicLookup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/lookup", nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, user.Email, body["email"])
	assert.Equal(t, map[string]interface{}{
		"email":        user.Email,
		"munchkinHash": app.MunchkinHash(user.Email),
		"organizations": []interface{}{
			map[string]interface{}{
				"id":   org.ExternalID,
				"name": org.Name,
			},
		},
	}, body)
}

func Test_PublicLookup_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	cookie, err := sessionStore.Cookie("foouser")
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/lookup", nil)
	r.AddCookie(cookie)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func Test_Lookup_ProbeToken(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users/lookup", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Scope-Probe token=%s", org.ProbeToken))

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"organizationID": org.ID}, body)

	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 1)
}

func Test_Lookup_Admin(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	require.NoError(t, database.SetUserAdmin(context.Background(), user.ID, true))

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/private/api/users/admin", nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"adminID": user.ID}, body)
}

func Test_Lookup_Admin_Unauthorized(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/private/api/users/admin", nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
