package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	// Use the org, so that firstProbeUpdateAt is set.
	org, err := db.FindOrganizationByProbeToken(org.ProbeToken)
	require.NoError(t, err)
	require.NotNil(t, org.FirstProbeUpdateAt)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/lookup", nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, user.Email, body["email"])
	assert.Equal(t, map[string]interface{}{
		"email": user.Email,
		"organizations": []interface{}{
			map[string]interface{}{
				"id":                 org.ExternalID,
				"name":               org.Name,
				"firstProbeUpdateAt": org.FirstProbeUpdateAt.UTC().Format(time.RFC3339),
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

	organizations, err := db.ListOrganizationsForUserIDs(user.ID)
	require.NoError(t, err)
	require.Len(t, organizations, 1)
	assert.NotNil(t, organizations[0].FirstProbeUpdateAt)
	assert.False(t, organizations[0].FirstProbeUpdateAt.IsZero())
}

func Test_Lookup_Admin(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getApprovedUser(t)
	require.NoError(t, db.SetUserAdmin(user.ID, true))

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

	user := getApprovedUser(t)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/private/api/users/admin", nil)

	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
