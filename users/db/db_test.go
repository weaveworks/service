package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/db/filter"
)

func TestDB_RemoveOtherUsersAccess(t *testing.T) {
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

func TestDB_AddFeatureFlag(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	_, org := dbtest.GetOrg(t, db)
	err := db.AddFeatureFlag(context.Background(), org.ExternalID, "supercow")
	require.NoError(t, err)

	org, err = db.FindOrganizationByProbeToken(context.Background(), org.ProbeToken)
	require.NoError(t, err)
	require.Equal(t, org.FeatureFlags, []string{"supercow"})
}

func TestDB_SetFeatureFlags(t *testing.T) {
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

// TestDB_ListByFeatureFlag shows that we can have ListOrganizations return
// only organizations that have a given feature flag set.
func TestDB_ListByFeatureFlag(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	_, org := dbtest.GetOrg(t, db)

	ctx := context.Background()
	flag := "foo"
	filterForFlag := filter.HasFeatureFlag(flag)
	{
		orgsWithFlag, err := db.ListOrganizations(ctx, filterForFlag, 0)
		require.NoError(t, err)
		assert.Equal(t, []*users.Organization{}, orgsWithFlag)
	}

	db.AddFeatureFlag(ctx, org.ExternalID, flag)
	org, err := db.FindOrganizationByID(ctx, org.ExternalID)
	require.NoError(t, err)
	{

		orgsWithFlag, err := db.ListOrganizations(ctx, filterForFlag, 0)
		require.NoError(t, err)
		assert.Equal(t, []*users.Organization{org}, orgsWithFlag)
	}
}

func TestDB_FindOrganizationByInternalID(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()

	u, err := db.CreateUser(ctx, "joe@email.com")

	if err != nil {
		t.Fatal(err)
	}

	o, err := db.CreateOrganization(ctx, u.ID, "happy-place-67", "My cool Org", "1234")

	if err != nil {
		t.Fatal(err)
	}

	org, err := db.FindOrganizationByInternalID(ctx, o.ID)

	if err != nil {
		t.Fatal(err)
	}

	if org.ID != o.ID {
		t.Fatalf("Expected ID to equal: %v; Actual: %v", o.ID, org.ID)
	}
}

func TestDB_FindGCP(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()

	externalAccountID := "E-XTERNAL-ACC-ID"
	u, err := db.CreateUser(ctx, "joe@weave.test")
	assert.NoError(t, err)
	org, err := db.CreateOrganizationWithGCP(ctx, u.ID, externalAccountID)
	assert.NoError(t, err)
	err = db.UpdateGCP(ctx, externalAccountID, "project_number:123", "partnerSubscriptions/1", "enterprise", string(partner.Active))
	assert.NoError(t, err)

	// Database request returns same values
	gcp, err := db.FindGCP(ctx, externalAccountID)
	assert.NoError(t, err)
	assert.Equal(t, externalAccountID, gcp.ExternalAccountID)
	assert.Equal(t, "project_number:123", gcp.ConsumerID)
	assert.Equal(t, "partnerSubscriptions/1", gcp.SubscriptionName)
	assert.Equal(t, "enterprise", gcp.SubscriptionLevel)
	assert.EqualValues(t, partner.Active, gcp.SubscriptionStatus)

	// FindOrganization returns same GCP as FindGCP
	neworg, err := db.FindOrganizationByID(ctx, org.ExternalID)
	assert.NoError(t, err)
	assert.Equal(t, gcp, neworg.GCP)
}
