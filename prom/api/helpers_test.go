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
)

var (
	app      *api.API
	database db.DB
	counter  int
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

// requestAsUser makes a request to the configs API as the given org and user.
func requestAsUser(t *testing.T, orgID, userID, method, urlStr string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r, err := http.NewRequest(method, urlStr, body)
	require.NoError(t, err)

	// inject org ID and set userID header
	r = r.WithContext(user.Inject(r.Context(), orgID))
	user.InjectIntoHTTPRequest(r.Context(), r)
	r.Header.Set("X-Scope-UserID", userID)

	app.ServeHTTP(w, r)
	return w
}
