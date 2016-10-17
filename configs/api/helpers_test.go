package api_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/configs/api"
	"github.com/weaveworks/service/configs/db"
	"github.com/weaveworks/service/configs/db/dbtest"
)

var (
	app      *api.API
	database db.DB
	counter  int
)

// setup sets up the environment for the tests.
func setup(t *testing.T) {
	app = api.New(api.DefaultConfig())
	database = dbtest.Setup(t)
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
func requestAsUser(t *testing.T, userID api.UserID, method, urlStr string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r, err := http.NewRequest(method, urlStr, body)
	require.NoError(t, err)
	r.Header.Add(app.UserIDHeader, string(userID))
	app.ServeHTTP(w, r)
	return w
}

// requestAsOrg makes a request to the configs API as the given user.
func requestAsOrg(t *testing.T, userID api.OrgID, method, urlStr string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r, err := http.NewRequest(method, urlStr, body)
	require.NoError(t, err)
	r.Header.Add(app.OrgIDHeader, string(userID))
	app.ServeHTTP(w, r)
	return w
}

// makeUserID makes an arbitrary user ID. Guaranteed to be unique within a test.
func makeUserID() api.UserID {
	counter++
	return api.UserID(fmt.Sprintf("user%d", counter))
}

// makeOrgID makes an arbitrary organization ID. Guaranteed to be unique within a test.
func makeOrgID() api.OrgID {
	counter++
	return api.OrgID(fmt.Sprintf("org%d", counter))
}

// makeSubsystem makes an arbitrary name for a subsystem.
func makeSubsystem() api.Subsystem {
	counter++
	return api.Subsystem(fmt.Sprintf("subsystem%d", counter))
}
