package db_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/common/constants/webhooks"
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

	// first, test two team name edge cases
	_, err := db.CreateTeam(ctx, "")
	require.Error(t, err)

	team, err := db.CreateTeam(ctx, "A-Team")
	require.NoError(t, err)

	user, org := dbtest.GetOrgForTeam(t, db, team)
	otherUser := dbtest.GetUser(t, db)

	userTeams, err := db.ListTeamsForUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, userTeams, 1)
	require.Equal(t, team.ID, userTeams[0].ID)

	otherUser, _, err = db.InviteUser(ctx, otherUser.Email, org.ExternalID)
	require.NoError(t, err)
	otherUserOrganizations, err := db.ListOrganizationsForUserIDs(context.Background(), otherUser.ID)
	require.NoError(t, err)
	require.Len(t, otherUserOrganizations, 1)

	otherUserTeams, err := db.ListTeamsForUserID(ctx, otherUser.ID)
	require.NoError(t, err)
	require.Len(t, otherUserTeams, 1)
	require.Equal(t, team.ID, otherUserTeams[0].ID)

	orgUsers, err := db.ListOrganizationUsers(ctx, org.ExternalID)
	require.NoError(t, err)
	require.Len(t, orgUsers, 2)

	teamUsers, err := db.ListTeamUsers(ctx, team.ID)
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

	o, err := db.CreateOrganization(ctx, u.ID, "happy-place-67", "My cool Org", "1234", "", u.TrialExpiresAt())

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
	org, err := db.CreateOrganizationWithGCP(ctx, u.ID, externalAccountID, u.TrialExpiresAt())
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
	require.NoError(t, err)

	user, _ := dbtest.GetOrgForTeam(t, db, team)

	userTeams, err := db.ListTeamsForUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, userTeams, 1)
	require.Equal(t, team.ID, userTeams[0].ID)

	// re-add user to the same team
	err = db.AddUserToTeam(ctx, user.ID, team.ID)
	require.NoError(t, err)
}

func Test_DB_CreateOrganizationWithTeam(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()
	user := dbtest.GetUser(t, db)

	externalID, err := db.GenerateOrganizationExternalID(ctx)
	require.NoError(t, err)
	name := strings.Replace(externalID, "-", " ", -1)

	_, err = db.CreateOrganizationWithTeam(ctx, user.ID, externalID, name, "", "non-existent", "", user.TrialExpiresAt())
	require.Error(t, err)

	teamName := fmt.Sprintf("%v Team", name)
	org, err := db.CreateOrganizationWithTeam(ctx, user.ID, externalID, name, "", "", teamName, user.TrialExpiresAt())
	require.NoError(t, err)
	require.Equal(t, name, org.Name)
	require.NotEqual(t, org.TeamID, "")

	teamUsers, err := db.ListTeamUsers(ctx, org.TeamID)
	require.NoError(t, err)
	require.Len(t, teamUsers, 1)
	require.Equal(t, teamUsers[0].ID, user.ID)

	teams, err := db.ListTeamsForUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, teams, 1)
	require.Equal(t, teams[0].ID, org.TeamID)
	require.Equal(t, teams[0].Name, teamName)
}

func TestDB_ListOrganizationWebhooks(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()

	u, err := db.CreateUser(ctx, "joe@email.com")
	assert.NoError(t, err)
	o, err := db.CreateOrganization(ctx, u.ID, "happy-place-67", "My cool Org", "1234", "", u.TrialExpiresAt())
	assert.NoError(t, err)

	w1, err := db.CreateOrganizationWebhook(ctx, o.ExternalID, webhooks.GithubPushIntegrationType)
	assert.NoError(t, err)
	w2, err := db.CreateOrganizationWebhook(ctx, o.ExternalID, webhooks.GithubPushIntegrationType)
	assert.NoError(t, err)

	ws, err := db.ListOrganizationWebhooks(ctx, o.ExternalID)
	assert.NoError(t, err)

	assert.Equal(t, []*users.Webhook{w1, w2}, ws)
}

func TestDB_CreateOrganizationWebhook(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()

	u, err := db.CreateUser(ctx, "joe@email.com")
	assert.NoError(t, err)
	o, err := db.CreateOrganization(ctx, u.ID, "happy-place-67", "My cool Org", "1234", "", u.TrialExpiresAt())
	assert.NoError(t, err)

	w, err := db.CreateOrganizationWebhook(ctx, o.ExternalID, webhooks.GithubPushIntegrationType)
	assert.NoError(t, err)

	id, err := strconv.Atoi(w.ID)
	assert.NoError(t, err)
	assert.True(t, id >= 1)

	assert.Equal(t, o.ID, w.OrganizationID)
	assert.Equal(t, webhooks.GithubPushIntegrationType, w.IntegrationType)
	assert.NotEmpty(t, w.SecretID)
	assert.NotEmpty(t, w.SecretSigningKey)
	assert.NotZero(t, w.CreatedAt)
	assert.Zero(t, w.DeletedAt)

	ws, err := db.ListOrganizationWebhooks(ctx, o.ExternalID)
	assert.NoError(t, err)

	assert.Equal(t, []*users.Webhook{w}, ws)
}

func TestDB_DeleteOrganizationWebhook(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()

	u, err := db.CreateUser(ctx, "joe@email.com")
	assert.NoError(t, err)
	o, err := db.CreateOrganization(ctx, u.ID, "happy-place-67", "My cool Org", "1234", "", u.TrialExpiresAt())
	assert.NoError(t, err)

	w1, err := db.CreateOrganizationWebhook(ctx, o.ExternalID, webhooks.GithubPushIntegrationType)
	assert.NoError(t, err)
	w2, err := db.CreateOrganizationWebhook(ctx, o.ExternalID, webhooks.GithubPushIntegrationType)
	assert.NoError(t, err)

	err = db.DeleteOrganizationWebhook(ctx, o.ExternalID, w1.SecretID)
	assert.NoError(t, err)

	ws, err := db.ListOrganizationWebhooks(ctx, o.ExternalID)
	assert.NoError(t, err)

	assert.Equal(t, []*users.Webhook{w2}, ws)
}

func TestDB_FindOrganizationWebhookBySecretID(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()

	u, err := db.CreateUser(ctx, "joe@email.com")
	assert.NoError(t, err)
	o, err := db.CreateOrganization(ctx, u.ID, "happy-place-67", "My cool Org", "1234", "", u.TrialExpiresAt())
	assert.NoError(t, err)

	_, err = db.CreateOrganizationWebhook(ctx, o.ExternalID, webhooks.GithubPushIntegrationType)
	assert.NoError(t, err)
	w2, err := db.CreateOrganizationWebhook(ctx, o.ExternalID, webhooks.GithubPushIntegrationType)
	assert.NoError(t, err)

	// Test valid SecretID
	w, err := db.FindOrganizationWebhookBySecretID(ctx, w2.SecretID)
	assert.NoError(t, err)
	assert.Equal(t, w2, w)

	// Test invalid SecretID
	w, err = db.FindOrganizationWebhookBySecretID(ctx, w2.SecretID+"a")
	assert.Error(t, err)
}

func TestDB_SetOrganizationWebhookFirstSeenAt(t *testing.T) {
	db := dbtest.Setup(t)
	defer dbtest.Cleanup(t, db)

	ctx := context.Background()

	u, err := db.CreateUser(ctx, "joe@email.com")
	assert.NoError(t, err)
	o, err := db.CreateOrganization(ctx, u.ID, "happy-place-67", "My cool Org", "1234", "", u.TrialExpiresAt())
	assert.NoError(t, err)

	w, err := db.CreateOrganizationWebhook(ctx, o.ExternalID, webhooks.GithubPushIntegrationType)
	assert.NoError(t, err)
	assert.Empty(t, w.FirstSeenAt)

	ti, err := db.SetOrganizationWebhookFirstSeenAt(ctx, w.SecretID)
	assert.NoError(t, err)

	w, err = db.FindOrganizationWebhookBySecretID(ctx, w.SecretID)
	assert.NoError(t, err)
	assert.NotEmpty(t, w.FirstSeenAt)
	assert.Equal(t, ti, w.FirstSeenAt)
}
