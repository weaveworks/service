package main

import (
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
	_, err = storage.ApproveUser(approved.ID)
	assert.NoError(t, err)
	// unapproved user1
	user1, err := storage.CreateUser("user1@weave.works")
	assert.NoError(t, err)
	// unapproved user2
	user2, err := storage.CreateUser("user2@weave.works")
	assert.NoError(t, err)

	// List unapproved users
	// should equal user1, user2
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/private/api/users", nil)
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
	// user1 should be approved
	assert.False(t, found.ApprovedAt.IsZero())
	// user1 should have an organization
	assert.NotEqual(t, "", found.OrganizationID)
	assert.NotEqual(t, "", found.OrganizationName)

	// Email should have been sent
	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{user1.Email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "Your Scope account has been approved!")
		assert.Contains(t, string(sentEmails[0].HTML), "Your Scope account has been approved!")
	}

	// List unapproved users
	// should equal user2
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "/private/api/users", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), fmt.Sprintf(`<form action="/private/api/users/%s/approve" method="POST">`, approved.ID))
	assert.NotContains(t, w.Body.String(), fmt.Sprintf(`<form action="/private/api/users/%s/approve" method="POST">`, user1.ID))
	assert.Contains(t, w.Body.String(), fmt.Sprintf(`<form action="/private/api/users/%s/approve" method="POST">`, user2.ID))
}
