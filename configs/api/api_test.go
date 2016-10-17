package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

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

// configs returns 401 to requests without authentication.
func Test_GetConfig_Anonymous(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/configs/made-up-service", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
