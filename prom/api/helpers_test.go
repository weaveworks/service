package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/prom/api"
	"github.com/weaveworks/service/prom/db"
	"github.com/weaveworks/service/prom/db/dbtest"
	"github.com/weaveworks/service/users/sessions"
)

var (
	app          *api.API
	database     db.DB
	counter      int
	sessionStore sessions.Store
)

// setup sets up the environment for the tests.
func setup(t *testing.T) {
	database = dbtest.Setup(t)
	app = api.New(database)
	counter = 0
}

// cleanup cleans up the environment after a test.
func cleanup(t *testing.T) {
	dbtest.Cleanup(t, database)
}

// request makes a request to the configs API.
func request(t *testing.T, method, urlStr string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r, err := http.NewRequest(method, urlStr, body)
	require.NoError(t, err)
	app.ServeHTTP(w, r)
	return w
}

// requestAsUser makes a request to the configs API as the given user.
func requestAsUser(t *testing.T, orgID, userID, method, urlStr string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r, err := http.NewRequest(method, urlStr, body)
	require.NoError(t, err)
	r = r.WithContext(user.Inject(r.Context(), orgID))
	user.InjectIntoHTTPRequest(r.Context(), r)
	r.Header.Set("X-Scope-UserID", userID)
	app.ServeHTTP(w, r)
	return w
}

// // RequestAs makes a request as the given user.
// func requestAs(t *testing.T, userID string, method, urlStr string, body io.Reader) *httptest.ResponseRecorder {
// 	cookie, err := sessionStore.Cookie(userID)
// 	assert.NoError(t, err)

// 	w := httptest.NewRecorder()
// 	r, err := http.NewRequest(method, urlStr, body)
// 	r.AddCookie(cookie)
// 	require.NoError(t, err)
// 	app.ServeHTTP(w, r)
// 	return w

// 	// r, err := http.NewRequest(method, endpoint, body)
// 	// require.NoError(t, err)

// 	// r.AddCookie(cookie)
// 	// return r
// }
