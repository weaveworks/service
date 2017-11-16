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

func TestDB_RemoveOtherUsersAccessWithTeams(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()

	team, err := db.CreateTeam(ctx, "A-Team")
	assert.NoError(t, err)

	user, org := dbtest.GetOrgForTeam(t, db, team)
	otherUser := dbtest.GetUser(t, db)

	userTeams, err := db.ListTeamsForUserID(ctx, user.ID)
	assert.NoError(t, err)
	require.Len(t, userTeams, 1)
	assert.Equal(t, team.ID, userTeams[0].ID)

	otherUser, _, err = db.InviteUser(ctx, otherUser.Email, org.ExternalID)
	require.NoError(t, err)
	otherUserOrganizations, err := db.ListOrganizationsForUserIDs(context.Background(), otherUser.ID)
	require.NoError(t, err)
	require.Len(t, otherUserOrganizations, 1)

	otherUserTeams, err := db.ListTeamsForUserID(ctx, otherUser.ID)
	assert.NoError(t, err)
	require.Len(t, otherUserTeams, 1)
	assert.Equal(t, team.ID, otherUserTeams[0].ID)

	orgUsers, err := db.ListOrganizationUsers(ctx, org.ExternalID)
	require.NoError(t, err)
	require.Len(t, orgUsers, 2)

	teamUsers, err := db.ListTeamUsers(ctx, team.ID)
	t.Log(team.ID)
	require.NoError(t, err)
	require.Len(t, teamUsers, 2)

	err = db.RemoveUserFromOrganization(ctx, org.ExternalID, otherUser.Email)
	require.NoError(t, err)

	orgUsers, err = db.ListOrganizationUsers(ctx, org.ExternalID)
	require.NoError(t, err)
	require.Len(t, orgUsers, 1)

	teamUsers, err = db.ListTeamUsers(ctx, team.ID)
	require.NoError(t, err)
	require.Len(t, teamUsers, 1)
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

func TestDB_LastLoginTimestamp(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()

	u, err := db.CreateUser(ctx, "james@weave.test")
	assert.NoError(t, err)
	assert.Equal(t, u.FirstLoginAt.IsZero(), true)
	assert.Equal(t, u.LastLoginAt.IsZero(), true)

	err = db.SetUserLastLoginAt(ctx, u.ID)
	assert.NoError(t, err)

	// reload user
	u, err = db.FindUserByID(ctx, u.ID)
	assert.NoError(t, err)
	assert.Equal(t, u.FirstLoginAt.IsZero(), false)
	assert.Equal(t, u.LastLoginAt.IsZero(), false)
	assert.Equal(t, u.FirstLoginAt.Equal(u.LastLoginAt), true)

	// keep a copy
	firstLoginAt := u.FirstLoginAt

	err = db.SetUserLastLoginAt(ctx, u.ID)
	assert.NoError(t, err)

	// reload user, again
	u, err = db.FindUserByID(ctx, u.ID)
	assert.NoError(t, err)
	assert.Equal(t, u.FirstLoginAt.IsZero(), false)
	assert.Equal(t, u.LastLoginAt.IsZero(), false)
	assert.Equal(t, u.FirstLoginAt.Equal(firstLoginAt), true)
	assert.Equal(t, u.LastLoginAt.After(u.FirstLoginAt), true)
}

func TestDB_DoubleTeamMembershipEntry(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()

	team, err := db.CreateTeam(ctx, "A-Team")
	assert.NoError(t, err)

	user, _ := dbtest.GetOrgForTeam(t, db, team)

	userTeams, err := db.ListTeamsForUserID(ctx, user.ID)
	assert.NoError(t, err)
	require.Len(t, userTeams, 1)
	assert.Equal(t, team.ID, userTeams[0].ID)

	// re-add user to the same team
	err = db.AddUserToTeam(ctx, user.ID, team.ID)
	assert.NoError(t, err)
}
