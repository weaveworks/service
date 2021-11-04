package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/login"
)

func TestAPI_User_UpdateUser(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getUser(t)
	assert.Equal(t, "", user.Company)
	assert.Equal(t, "", user.Name)

	t.Run("update all fields", func(t *testing.T) {
		user = getUser(t)
		dbtest.AddLogin(t, database, user)
		logins.SetUsers(map[string]login.Claims{user.Email: {Email: user.Email}})
		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/logins/mock/attach?code="+user.Email, nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)

		body, _ := json.Marshal(map[string]string{
			"company":   "Evil Corp",
			"name":      "Dave DAVE",
			"firstName": "Dave",
			"lastName":  "DAVE",
		})
		r = requestAs(t, user, "PUT", "/api/users/user", bytes.NewReader(body))
		app.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		user, err := database.FindUserByEmail(context.TODO(), user.Email)
		assert.NoError(t, err)
		assert.Equal(t, "Dave", user.FirstName)
		assert.Equal(t, "DAVE", user.LastName)
		assert.Equal(t, "Dave DAVE", user.Name)
		assert.Equal(t, "Evil Corp", user.Company)
	})

	t.Run("update single field", func(t *testing.T) {
		user = getUser(t)
		dbtest.AddLogin(t, database, user)
		logins.SetUsers(map[string]login.Claims{user.Email: {Email: user.Email}})
		w := httptest.NewRecorder()
		body, _ := json.Marshal(map[string]string{"company": "Wayne Enterprises"})
		r := requestAs(t, user, "PUT", "/api/users/user", bytes.NewReader(body))
		app.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		user, err := database.FindUserByEmail(context.TODO(), user.Email)
		assert.NoError(t, err)
		assert.Equal(t, "Wayne Enterprises", user.Company)
	})

	t.Run("update strips HTML", func(t *testing.T) {
		user = getUser(t)
		dbtest.AddLogin(t, database, user)
		logins.SetUsers(map[string]login.Claims{user.Email: {Email: user.Email}})
		w := httptest.NewRecorder()
		body, _ := json.Marshal(map[string]string{
			"company":   "<img>Evil Corp",
			"name":      "Dave <img>DAVE",
			"firstName": "Dave<img>",
			"lastName":  "DA<img>VE",
		})
		r := requestAs(t, user, "PUT", "/api/users/user", bytes.NewReader(body))
		app.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		user, err := database.FindUserByEmail(context.TODO(), user.Email)
		assert.NoError(t, err)
		assert.Equal(t, "Dave", user.FirstName)
		assert.Equal(t, "DAVE", user.LastName)
		assert.Equal(t, "Dave DAVE", user.Name)
		assert.Equal(t, "Evil Corp", user.Company)
	})
	t.Run("invalid name", func(t *testing.T) {
		user = getUser(t)
		dbtest.AddLogin(t, database, user)
		logins.SetUsers(map[string]login.Claims{user.Email: {Email: user.Email}})
		w := httptest.NewRecorder()
		body, _ := json.Marshal(map[string]string{"company": strings.Repeat("x", 120)})
		r := requestAs(t, user, "PUT", "/api/users/user", bytes.NewReader(body))
		app.ServeHTTP(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		user, err := database.FindUserByEmail(context.TODO(), user.Email)
		assert.NoError(t, err)
		assert.Equal(t, "", user.Company)
	})
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
	assert.Equal(t, user.FirstName, resp.FirstName)
	assert.Equal(t, user.LastName, resp.LastName)
}
