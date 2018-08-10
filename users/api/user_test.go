package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users"
)

func TestAPI_User_UpdateUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	assert.Equal(t, "", user.Company)
	assert.Equal(t, "", user.Name)

	{ // update all fields
		user = getUser(t)
		w := httptest.NewRecorder()
		body, _ := json.Marshal(map[string]string{
			"company": "Evil Corp",
			"name":    "Dave",
		})
		r := requestAs(t, user, "PUT", "/api/users/user", bytes.NewReader(body))
		app.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp *users.User
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		assert.Equal(t, "Dave", resp.Name)
		assert.Equal(t, "Evil Corp", resp.Company)
	}

	{ // update single field
		user = getUser(t)
		w := httptest.NewRecorder()
		body, _ := json.Marshal(map[string]string{"company": "Wayne Enterprises"})
		r := requestAs(t, user, "PUT", "/api/users/user", bytes.NewReader(body))
		app.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp *users.User
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		assert.Equal(t, user.Name, resp.Name)
		assert.Equal(t, "Wayne Enterprises", resp.Company)
	}
}

func TestAPI_User_GetCurrentUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/user", nil)

	app.ServeHTTP(w, r)

	var resp *users.UserResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, user.Email, resp.Email)
	assert.Equal(t, user.Company, resp.Company)
	assert.Equal(t, user.Name, resp.Name)
}
