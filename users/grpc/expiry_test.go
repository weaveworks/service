package grpc_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/dbtest"
)

func makeOrgWithTrialExpired(t *testing.T) *users.Organization {
	_, org := dbtest.GetOrg(t, database)

	_, err := server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "RefuseDataAccess",
			Value:      true,
		})
	require.NoError(t, err)
	_, err = server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "RefuseDataUpload",
			Value:      true,
		})
	require.NoError(t, err)

	newOrg, err := database.FindOrganizationByID(ctx, org.ExternalID)
	require.NoError(t, err)
	return newOrg
}

func Test_GetExpiry(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create two orgs, one expired and one not
	_, orgA := dbtest.GetOrg(t, database)
	orgB := makeOrgWithTrialExpired(t)

	{
		resp, err := server.GetDataExpiry(ctx, &users.DataExpiryRequest{OrganizationID: orgA.ID})
		require.NoError(t, err)
		expiry := time.Time{} // zero value
		assert.Equal(t, &users.DataExpiryResponse{ExpireBefore: &expiry}, resp)
	}
	{
		resp, err := server.GetDataExpiry(ctx, &users.DataExpiryRequest{OrganizationID: orgB.ID})
		require.NoError(t, err)
		expiry := orgB.TrialExpiresAt.Add(time.Hour * 24 * 7 * 2) // TODO: less hard-coded
		assert.Equal(t, &users.DataExpiryResponse{ExpireBefore: &expiry}, resp)
	}
}
