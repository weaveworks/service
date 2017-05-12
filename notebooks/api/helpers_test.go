package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/notebooks/api"
	"github.com/weaveworks/service/notebooks/db"
	"github.com/weaveworks/service/notebooks/db/dbtest"
	users "github.com/weaveworks/service/users/client"
)

var (
	app      *api.API
	database db.DB
	counter  int
)

// setup sets up the environment for the tests.
func setup(t *testing.T) {
	database = dbtest.Setup(t)
	mockUsersClient, _ := users.New("mock", "", users.CachingClientConfig{})
	app = api.New(database, mockUsersClient)
	counter = 0
}

// cleanup cleans up the environment after a test.
func cleanup(t *testing.T) {
	dbtest.Cleanup(t, database)
}

// requestAsUser makes a request to the configs API as the given org and user.
func requestAsUser(t *testing.T, orgID, userID, method, urlStr string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r, err := http.NewRequest(method, urlStr, body)
	require.NoError(t, err)

	// inject org ID and set userID header
	r = r.WithContext(user.InjectOrgID(r.Context(), orgID))
	user.InjectOrgIDIntoHTTPRequest(r.Context(), r)
	r.Header.Set("X-Scope-UserID", userID)

	app.ServeHTTP(w, r)
	return w
}
