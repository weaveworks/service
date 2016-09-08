package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users"
)

func Test_APITokens_CreateAndUseAPIToken(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user := getApprovedUser(t)
	org1 := createOrgForUser(t, user)
	org2 := createOrgForUser(t, user)

	// List my tokens
	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/tokens", nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "{\"tokens\":[]}\n", w.Body.String())
	}

	// Create a token
	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "POST", "/api/users/tokens", jsonBody{"description": "my awesome token"}.Reader(t))
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusCreated, w.Code)
		body := map[string]interface{}{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		tokens, err := db.ListAPITokensForUserIDs(user.ID)
		require.NoError(t, err)
		require.Len(t, tokens, 1)
		assert.Equal(t, map[string]interface{}{
			"token":       tokens[0].Token,
			"description": "my awesome token",
		}, body)
	}

	// Refresh the user
	tokens, err := db.ListAPITokensForUserIDs(user.ID)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	token := tokens[0].Token

	// List tokens again with some
	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/tokens", nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		body := map[string]interface{}{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, map[string]interface{}{
			"tokens": []interface{}{
				map[string]interface{}{
					"token":       token,
					"description": tokens[0].Description,
				},
			},
		}, body)
	}

	// Use a token to authenticate
	{
		w := httptest.NewRecorder()
		r, err := http.NewRequest("GET", "/api/users/lookup", nil)
		require.NoError(t, err)
		r.Header.Set("Authorization", fmt.Sprintf("Scope-User token=%s", token))

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		body := map[string]interface{}{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, user.Email, body["email"])

		// It should give access to all instances
		assert.Equal(t, map[string]interface{}{
			"email": user.Email,
			"organizations": []interface{}{
				map[string]interface{}{
					"id":   org1.ExternalID,
					"name": org1.Name,
				},
				map[string]interface{}{
					"id":   org2.ExternalID,
					"name": org2.Name,
				},
			},
		}, body)
	}

	// Delete a token
	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "DELETE", "/api/users/tokens/"+token, nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNoContent, w.Code)
		_, err := db.FindUserByAPIToken(token)
		assert.EqualError(t, err, users.ErrNotFound.Error())
	}

	// Deleted tokens should no longer work
	{
		w := httptest.NewRecorder()
		r, err := http.NewRequest("GET", "/api/users/lookup", nil)
		require.NoError(t, err)
		r.Header.Set("Authorization", fmt.Sprintf("Scope-User token=%s", token))
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}

	// Refresh the user (should have no more tokens)
	tokens, err = db.ListAPITokensForUserIDs(user.ID)
	require.NoError(t, err)
	require.Len(t, tokens, 0)

	// List my tokens (deleted ones should not be listed)
	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/tokens", nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "{\"tokens\":[]}\n", w.Body.String())
	}
}
