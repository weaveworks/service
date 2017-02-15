package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/client"
)

func findLoginLink(t *testing.T, e *email.Email) (url, token string) {
	pattern := domain + `/login/[\w.%]+/([A-Za-z0-9%._=-]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(string(e.Text))
	require.Len(t, matches, 2, fmt.Sprintf("Could not find Login Link in text: %q", e.Text))
	require.NotEqual(t, "", matches[0])
	require.NotEqual(t, "", matches[1])
	require.Contains(t, string(e.HTML), matches[0], fmt.Sprintf("Could not find Login Link in html: %q", e.HTML))
	return matches[0], matches[1]
}

func newLoginRequest(t *testing.T, e *email.Email) *http.Request {
	loginLink, _ := findLoginLink(t, e)
	require.Contains(t, string(e.HTML), loginLink)

	u, err := url.Parse(loginLink)
	require.NoError(t, err)
	// convert email link /login/foo/bar to /api/users/login?email=foo&token=bar
	fragments := strings.Split(u.Path, "/")
	params := url.Values{}
	params.Set("email", fragments[2])
	params.Set("token", fragments[3])
	path := fmt.Sprintf("/api/users/login?%s", params.Encode())
	r, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)
	return r
}

// Check if a response has some named cookie
func hasCookie(w *httptest.ResponseRecorder, name string) bool {
	cookies := (&http.Response{Header: w.HeaderMap}).Cookies()
	for _, c := range cookies {
		if c.Name == name {
			return true
		}
	}
	return false
}

func Test_Signup(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := "joe@weave.works"

	// Signup as a new user, should send login email
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", jsonBody{"email": email}.Reader(t))
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"mailSent": true, "email": email}, body)
	require.Len(t, sentEmails, 1)
	user, err := database.FindUserByEmail(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, []string{email}, sentEmails[0].To)
	loginLink, emailToken := findLoginLink(t, sentEmails[0])
	assert.Contains(t, string(sentEmails[0].HTML), loginLink)

	// Check the db one was hashed
	assert.NotEqual(t, "", user.Token, "user should have a token set")
	assert.NotEqual(t, user.Token, emailToken, "stored token should have been hashed")

	// Check the email one wasn't hashed (by looking for dollar-signs)
	assert.NotContains(t, emailToken, "$")
	assert.NotContains(t, emailToken, "%24")

	// Login with the link
	u, err := url.Parse(loginLink)
	assert.NoError(t, err)
	// convert email link /login/foo/bar to /api/users/login?email=foo&token=bar
	fragments := strings.Split(u.Path, "/")
	params := url.Values{}
	params.Set("email", fragments[2])
	params.Set("token", fragments[3])
	path := fmt.Sprintf("/api/users/login?%s", params.Encode())
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", path, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, client.AuthCookieName))
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"firstLogin":   true,
		"email":        user.Email,
		"munchkinHash": app.MunchkinHash(user.Email),
	}, body)

	user, err = database.FindUserByEmail(context.Background(), email)
	require.NoError(t, err)
	// Invalidates their login token
	assert.Equal(t, "", user.Token)
	// Sets their FirstLoginAt
	assert.False(t, user.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")
	firstLoginAt := user.FirstLoginAt
	// Doesn't create an organization.
	organizations, err := database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Len(t, organizations, 0)

	// Subsequent Logins do not change their FirstLoginAt or organization
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("POST", "/api/users/signup", jsonBody{"email": email}.Reader(t))
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, sentEmails, 2)
	assert.Equal(t, []string{email}, sentEmails[1].To)
	w = httptest.NewRecorder()
	app.ServeHTTP(w, newLoginRequest(t, sentEmails[1]))
	assert.Equal(t, http.StatusOK, w.Code)
	body = map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"email":        user.Email,
		"munchkinHash": app.MunchkinHash(user.Email),
	}, body)

	user, err = database.FindUserByEmail(context.Background(), email)
	require.NoError(t, err)
	assert.False(t, user.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")
	assert.Equal(t, firstLoginAt, user.FirstLoginAt, "Second login should not have changed user's FirstLoginAt")
	organizations, err = database.ListOrganizationsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Len(t, organizations, 0)
}

func Test_Signup_WithInvalidJSON(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := ""
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", strings.NewReader("this isn't json"))

	_, err := database.FindUserByEmail(context.Background(), email)
	assert.EqualError(t, err, users.ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid character 'h' in literal true (expecting 'r')")
	_, err = database.FindUserByEmail(context.Background(), email)
	assert.EqualError(t, err, users.ErrNotFound.Error())
}

func Test_Signup_WithBlankEmail(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := ""
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/api/users/signup", jsonBody{}.Reader(t))

	_, err := database.FindUserByEmail(context.Background(), email)
	assert.EqualError(t, err, users.ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Email cannot be blank")
	_, err = database.FindUserByEmail(context.Background(), email)
	assert.EqualError(t, err, users.ErrNotFound.Error())
}

func Test_Signup_ViaOAuth(t *testing.T) {
	setup(t)
	defer cleanup(t)

	email := "joe@example.com"
	logins.Register("mock", MockLoginProvider{
		"joe": {ID: "joe", Email: email},
	})

	// Signup as a new user via oauth, should *not* send welcome email
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/logins/mock/attach?code=joe&state=state", nil)
	_, err := database.FindUserByEmail(context.Background(), email)
	assert.EqualError(t, err, users.ErrNotFound.Error())
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, client.AuthCookieName))
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"firstLogin":   true,
		"userCreated":  true,
		"email":        email,
		"munchkinHash": app.MunchkinHash(email),
	}, body)
	assert.Len(t, sentEmails, 0)

	user, err := database.FindUserByEmail(context.Background(), email)
	require.NoError(t, err)

	assert.Equal(t, "", user.Token, "user should not have a token set")
	assert.False(t, user.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")

	// User should have login set
	userLogins, err := database.ListLoginsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	if assert.Len(t, userLogins, 1) {
		assert.Equal(t, user.ID, userLogins[0].UserID)
		assert.Equal(t, "mock", userLogins[0].Provider)
		assert.Equal(t, "joe", userLogins[0].ProviderID)
	}
}

func Test_Signup_ViaOAuth_MatchesByEmail(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	logins.Register("mock", MockLoginProvider{
		"joe": {ID: "joe", Email: user.Email},
	})
	// User should not have any logins yet.
	userLogins, err := database.ListLoginsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Len(t, userLogins, 0)

	// Signup as an existing user via oauth, should match with existing user
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/logins/mock/attach?code=joe&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, client.AuthCookieName))
	assert.Len(t, sentEmails, 0)

	found, err := database.FindUserByEmail(context.Background(), user.Email)
	require.NoError(t, err)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{
		"firstLogin":   true,
		"email":        user.Email,
		"munchkinHash": app.MunchkinHash(user.Email),
	}, body)

	assert.Equal(t, user.ID, found.ID, "user id should match the existing")
	assert.Equal(t, user.Token, found.Token, "user should still have same token set")
	assert.False(t, found.FirstLoginAt.IsZero(), "Login should have set user's FirstLoginAt")

	// User should have a login set
	foundLogins, err := database.ListLoginsForUserIDs(context.Background(), found.ID)
	require.NoError(t, err)
	if assert.Len(t, foundLogins, 1) {
		assert.Equal(t, user.ID, foundLogins[0].UserID)
		assert.Equal(t, "mock", foundLogins[0].Provider)
		assert.Equal(t, "joe", foundLogins[0].ProviderID)
	}
}

func Test_Signup_ViaOAuth_EmailChanged(t *testing.T) {
	// When a user has changed their remote email, but the remote user ID is the same.
	setup(t)
	defer cleanup(t)
	user := getUser(t)
	provider := MockLoginProvider{
		"joe": {ID: "joe", Email: user.Email},
	}
	logins.Register("mock", provider)

	require.NoError(t, database.AddLoginToUser(context.Background(), user.ID, "mock", "joe", nil))

	// Change the remote email
	newEmail := "fran@example.com"
	provider["joe"] = MockRemoteUser{ID: "joe", Email: newEmail}

	// Login as an existing user with remote email changed
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/logins/mock/attach?code=joe&state=state", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hasCookie(w, client.AuthCookieName))
	assert.Len(t, sentEmails, 0)

	_, err := database.FindUserByEmail(context.Background(), newEmail)
	assert.EqualError(t, err, users.ErrNotFound.Error())

	userLogins, err := database.ListLoginsForUserIDs(context.Background(), user.ID)
	require.NoError(t, err)
	// User should have a login set
	if assert.Len(t, userLogins, 1) {
		assert.Equal(t, user.ID, userLogins[0].UserID)
		assert.Equal(t, "mock", userLogins[0].Provider)
		assert.Equal(t, "joe", userLogins[0].ProviderID)
	}
}
