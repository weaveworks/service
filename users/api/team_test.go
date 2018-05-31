package api_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users/db/dbtest"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPI_deleteTeam(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org, _ := dbtest.GetOrgAndTeam(t, database)

	teams, err := database.ListTeamsForUserID(context.TODO(), user.ID)
	assert.NoError(t, err)
	assert.Len(t, teams, 1)

	{ // non-empty team
		w := httptest.NewRecorder()
		r := requestAs(t, user, "DELETE", "/api/users/teams/"+org.TeamExternalID, nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}
	// delete org
	err = database.DeleteOrganization(context.TODO(), org.ExternalID)
	assert.NoError(t, err)
	{ // now empty team
		w := httptest.NewRecorder()
		r := requestAs(t, user, "DELETE", "/api/users/teams/"+org.TeamExternalID, nil)
		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusNoContent, w.Code)
	}

	teams, err = database.ListTeamsForUserID(context.TODO(), user.ID)
	assert.NoError(t, err)
	assert.Len(t, teams, 0)
}
