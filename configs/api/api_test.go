package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/configs/api"
	"github.com/weaveworks/service/configs/db"
	"github.com/weaveworks/service/configs/db/dbtest"
)

var (
	app      *api.API
	database db.DB
)

// setup sets up the environment for the tests.
func setup(t *testing.T) {
	app = api.New(false)
	database = dbtest.Setup(t)
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

// The root page returns 200 OK.
func Test_Root_OK(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := request(t, "GET", "/", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

// configs returns 401 to requests without authentication.
func Test_GetConfig_Anonymous(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := request(t, "GET", "/api/configs/made-up-service", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
