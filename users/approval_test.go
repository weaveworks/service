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
	assert.False(t, found.ApprovedAt.IsZero(), "user should have an approved_at timestamp")
	assert.NotEqual(t, "", found.Organization.ID, "user should have an organization id")
	assert.NotEqual(t, "", found.Organization.Name, "user should have an organization name")
	assert.NotEqual(t, "", found.Organization.ProbeToken, "user should have a probe token")

	// Email should have been sent
	if assert.Len(t, sentEmails, 1) {
		assert.Equal(t, []string{user1.Email}, sentEmails[0].To)
		assert.Contains(t, string(sentEmails[0].Text), "Your Scope account has been approved!")
		assert.Contains(t, string(sentEmails[0].HTML), "Your Scope account has been approved!")
		assert.Contains(t, string(sentEmails[0].Text), found.Organization.ProbeToken)
		assert.Contains(t, string(sentEmails[0].HTML), found.Organization.ProbeToken)
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
