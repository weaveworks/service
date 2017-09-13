package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/db/filter"
)

func Test_DB_RemoveOtherUsersAccess(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	_, org := dbtest.GetOrg(t, db)
	otherUser := dbtest.GetUser(t, db)
	otherUser, _, err := db.InviteUser(context.Background(), otherUser.Email, org.ExternalID)
	require.NoError(t, err)
	otherUserOrganizations, err := db.ListOrganizationsForUserIDs(context.Background(), otherUser.ID)
	require.NoError(t, err)
	require.Len(t, otherUserOrganizations, 1)

	orgUsers, err := db.ListOrganizationUsers(context.Background(), org.ExternalID)
	require.NoError(t, err)
	require.Len(t, orgUsers, 2)

	err = db.RemoveUserFromOrganization(context.Background(), org.ExternalID, otherUser.Email)
	require.NoError(t, err)

	orgUsers, err = db.ListOrganizationUsers(context.Background(), org.ExternalID)
	require.NoError(t, err)
	require.Len(t, orgUsers, 1)
}

func Test_DB_AddFeatureFlag(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	_, org := dbtest.GetOrg(t, db)
	err := db.AddFeatureFlag(context.Background(), org.ExternalID, "supercow")
	require.NoError(t, err)

	org, err = db.FindOrganizationByProbeToken(context.Background(), org.ProbeToken)
	require.NoError(t, err)
	require.Equal(t, org.FeatureFlags, []string{"supercow"})
}

func Test_DB_SetFeatureFlags(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	_, org := dbtest.GetOrg(t, db)

	for _, flags := range [][]string{
		{"supercow", "superchicken"},
		{"superchicken"},
		{},
	} {
		err := db.SetFeatureFlags(context.Background(), org.ExternalID, flags)
		require.NoError(t, err)
		org, err = db.FindOrganizationByProbeToken(context.Background(), org.ProbeToken)
		require.NoError(t, err)
		require.Equal(t, flags, org.FeatureFlags)
	}
}

// Test_DB_ListByFeatureFlag shows that we can have ListOrganizations return
// only organizations that have a given feature flag set.
func Test_DB_ListByFeatureFlag(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	_, org := dbtest.GetOrg(t, db)

	ctx := context.Background()
	flag := "foo"
	filterForFlag := filter.Organization{FeatureFlags: []string{flag}}
	{
		orgsWithFlag, err := db.ListOrganizations(ctx, filterForFlag)
		require.NoError(t, err)
		assert.Equal(t, []*users.Organization{}, orgsWithFlag)
	}

	db.AddFeatureFlag(ctx, org.ExternalID, flag)
	org, err := db.FindOrganizationByID(ctx, org.ExternalID)
	require.NoError(t, err)
	{

		orgsWithFlag, err := db.ListOrganizations(ctx, filterForFlag)
		require.NoError(t, err)
		assert.Equal(t, []*users.Organization{org}, orgsWithFlag)
	}
}
