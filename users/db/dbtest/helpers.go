package dbtest

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/weaveworks/service/users/externalids"
	"github.com/weaveworks/service/users/login"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
)

// GetUser makes a randomly named user
func GetUser(t *testing.T, db db.DB) *users.User {
	return GetUserWithDomain(t, db, "domain.com")
}

// GetUserWithDomain makes a randomly named user with an address email finishing with the provided domain
func GetUserWithDomain(t *testing.T, db db.DB, domain string) *users.User {
	email := fmt.Sprintf("%d@%v", rand.Int63(), domain)
	user, err := db.CreateUser(context.Background(), email)
	require.NoError(t, err)
	return user
}

// SetUserInfo makes a random Name and Company name and updates the user
func AddUserInfoToUser(t *testing.T, db db.DB, user *users.User) *users.User {
	random := rand.Int63()
	user, err := db.UpdateUser(context.Background(), user.ID, &users.UserUpdate{
		Name:    fmt.Sprintf("Test User %d", random),
		Company: fmt.Sprintf("Test Company %d", random),
	})
	require.NoError(t, err)
	return user
}

// AddGoogleLoginToUser fakes a signup via Google OAuth
func AddGoogleLoginToUser(t *testing.T, db db.DB, userID string) *oauth2.Token {
	loginID := fmt.Sprintf("login_id_%v", userID)
	token := &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: fmt.Sprintf("access_token_%v", userID),
		Expiry:      time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
	}
	session, err := json.Marshal(login.OAuthUserSession{Token: token})
	require.NoError(t, err)
	db.AddLoginToUser(context.TODO(), userID, login.GoogleProviderID, loginID, session)
	return token
}

// CreateOrgForUser creates a new random organization for this user
func CreateOrgForUser(t *testing.T, db db.DB, u *users.User) *users.Organization {
	externalID, err := db.GenerateOrganizationExternalID(context.Background())
	require.NoError(t, err)

	name := strings.Replace(externalID, "-", " ", -1)
	org, err := db.CreateOrganization(context.Background(), u.ID, externalID, name, "", "", u.TrialExpiresAt())
	require.NoError(t, err)

	assert.NotEqual(t, "", org.ID)
	assert.NotEqual(t, "", org.Name)
	assert.Equal(t, externalID, org.ExternalID)

	return org
}

// CreateOrgForTeam creates a new random organization for this team
func CreateOrgForTeam(t *testing.T, db db.DB, u *users.User, team *users.Team) *users.Organization {
	assert.NotEqual(t, nil, team)
	assert.NotEqual(t, "", team.ID)

	err := db.AddUserToTeam(context.Background(), u.ID, team.ID)
	require.NoError(t, err)

	externalID, err := db.GenerateOrganizationExternalID(context.Background())
	require.NoError(t, err)

	name := strings.Replace(externalID, "-", " ", -1)
	org, err := db.CreateOrganizationWithTeam(context.Background(), u.ID, externalID, name, "", team.ExternalID, "", u.TrialExpiresAt())
	require.NoError(t, err)

	assert.NotEqual(t, "", org.ID)
	assert.NotEqual(t, "", org.Name)
	assert.Equal(t, externalID, org.ExternalID)

	return org
}

// CreateWebhookForOrg creates a new random webhook for this org
func CreateWebhookForOrg(t *testing.T, db db.DB, org *users.Organization, integrationType string) *users.Webhook {
	w, err := db.CreateOrganizationWebhook(context.Background(), org.ExternalID, integrationType)
	require.NoError(t, err)
	return w
}

// GetOrgWebhook gets a webhook by secretID
func GetOrgWebhook(t *testing.T, db db.DB, secretID string) *users.Webhook {
	w, err := db.FindOrganizationWebhookBySecretID(context.Background(), secretID)
	require.NoError(t, err)
	return w
}

// GetOrg makes org with a random ExternalID and user for testing
func GetOrg(t *testing.T, db db.DB) (*users.User, *users.Organization) {
	user := GetUser(t, db)
	org := CreateOrgForUser(t, db, user)
	return user, org
}

// GetTeam creates a team with a randomly generated name.
func GetTeam(t *testing.T, db db.DB) *users.Team {
	teamName := strings.Title(externalids.Generate())
	return GetTeamWithName(t, db, teamName)
}

// GetTeamWithName creates a team with the provided name.
func GetTeamWithName(t *testing.T, db db.DB, teamName string) *users.Team {
	team, err := db.CreateTeam(context.Background(), teamName)
	assert.NoError(t, err)
	return team
}

// GetOrgAndTeam makes org with a random ExternalID, user and a team for testing
func GetOrgAndTeam(t *testing.T, db db.DB) (*users.User, *users.Organization, *users.Team) {
	teamName := strings.Title(externalids.Generate())
	team := GetTeamWithName(t, db, teamName)
	team.Name = teamName
	user := GetUser(t, db)
	org := CreateOrgForTeam(t, db, user, team)
	return user, org, team
}

// GetOrgForTeam makes org with a random ExternalID, user and a team for testing
func GetOrgForTeam(t *testing.T, db db.DB, team *users.Team) (*users.User, *users.Organization) {
	user := GetUser(t, db)
	org := CreateOrgForTeam(t, db, user, team)
	return user, org
}
