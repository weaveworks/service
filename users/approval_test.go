package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Approval(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create some users
	// approved user
	approved, err := storage.CreateUser("approved@weave.works")
	assert.NoError(t, err)
	assert.NoError(t, storage.ApproveUser(approved.ID))
	// unapproved user1
	user1, err := storage.CreateUser("user1@weave.works")
	assert.NoError(t, err)
	// unapproved user2
	user2, err := storage.CreateUser("user2@weave.works")
	assert.NoError(t, err)

	// List unapproved users
	// should equal user1, user2
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/users/private/users", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := []userView{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, []userView{
		{ID: user1.ID, Email: user1.Email, CreatedAt: user1.CreatedAt},
		{ID: user2.ID, Email: user2.Email, CreatedAt: user2.CreatedAt},
	},
		body)

	// Approve user1
	w = httptest.NewRecorder()
	r, _ = http.NewRequest(
		"POST",
		fmt.Sprintf("/api/users/private/users/%s/approve", user1.ID),
		nil,
	)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/api/users/private/users", w.HeaderMap.Get("Location"))

	found, err := storage.FindUserByID(user1.ID)
	assert.NoError(t, err)
	// user1 should be approved
	assert.False(t, found.ApprovedAt.IsZero())
	// user1 should have an organization
	assert.NotEqual(t, "", found.OrganizationID)
	assert.NotEqual(t, "", found.OrganizationName)

	// List unapproved users
	// should equal user2
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "/api/users/private/users", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body = []userView{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, []userView{
		{ID: user2.ID, Email: user2.Email, CreatedAt: user2.CreatedAt},
	},
		body)
}
