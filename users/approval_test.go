package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Approval(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create some users
	// approved user
	approved, err := storage.CreateUser("approved@weave.works")
	require.NoError(t, err)
	approved, err = storage.ApproveUser(approved.ID)
	require.NoError(t, err)
	// unapproved user1
	user1, err := storage.CreateUser("user1@weave.works")
	require.NoError(t, err)
	// unapproved user2
	user2, err := storage.CreateUser("user2@weave.works")
	require.NoError(t, err)

	// List unapproved users
	// should equal user1, user2
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users?approved=false", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), fmt.Sprintf(`<form action="/private/api/users/%s/approve" method="POST">`, approved.ID))
	assert.Contains(t, w.Body.String(), fmt.Sprintf(`<form action="/private/api/users/%s/approve" method="POST">`, user1.ID))
	assert.Contains(t, w.Body.String(), fmt.Sprintf(`<form action="/private/api/users/%s/approve" method="POST">`, user2.ID))

	// Approve user1
	w = httptest.NewRecorder()
	r, _ = http.NewRequest(
		"POST",
		fmt.Sprintf("/private/api/users/%s/approve", user1.ID),
		nil,
	)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/private/api/users", w.HeaderMap.Get("Location"))

	found, err := storage.FindUserByID(user1.ID)
	assert.NoError(t, err)
	assert.False(t, found.ApprovedAt.IsZero(), "user should have an approved_at timestamp")
	assert.NotEqual(t, "", found.Organization.ID, "user should have an organization id")
	assert.NotEqual(t, "", found.Organization.Name, "user should have an organization name")
	assert.NotEqual(t, "", found.Organization.ProbeToken, "user should have a probe token")

	// Email should have been sent
	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{user1.Email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "account has been approved!")
		assert.Contains(t, string(sentEmails[0].HTML), "account has been approved!")
	}

	// List unapproved users
	// should equal user2
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "/private/api/users?approved=false", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), approved.Email)
	assert.NotContains(t, w.Body.String(), user1.Email)
	assert.Contains(t, w.Body.String(), fmt.Sprintf(`<form action="/private/api/users/%s/approve" method="POST">`, user2.ID))

	// List approved users
	// should equal approved and user1
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "/private/api/users?approved=true", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), approved.Email)
	assert.Contains(t, w.Body.String(), user1.Email)
	assert.NotContains(t, w.Body.String(), user2.Email)

	// List all users
	// should equal all users
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "/private/api/users", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), approved.Email)
	assert.Contains(t, w.Body.String(), user1.Email)
	assert.Contains(t, w.Body.String(), user2.Email)
}

func Test_Approval_IsIdempotent(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create a user
	user, err := storage.CreateUser("approved@weave.works")
	require.NoError(t, err)

	// Approve the user
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	organizationID := user.Organization.ID
	assert.NotEqual(t, "", organizationID)
	approvedAt := user.ApprovedAt
	assert.False(t, approvedAt.IsZero())

	// Approve them again
	user, err = storage.ApproveUser(user.ID)
	require.NoError(t, err)
	assert.Equal(t, organizationID, user.Organization.ID)
	assert.Equal(t, approvedAt, user.ApprovedAt)
}
