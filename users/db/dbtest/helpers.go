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

	"github.com/weaveworks/service/users/login"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
)

// GetUser makes a randomly named user
func GetUser(t *testing.T, db db.DB) *users.User {
	email := fmt.Sprintf("%d@weave.works", rand.Int63())
	user, err := db.CreateUser(context.Background(), email)
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
	org, err := db.CreateOrganization(context.Background(), u.ID, externalID, name, "")
	require.NoError(t, err)

	assert.NotEqual(t, "", org.ID)
	assert.NotEqual(t, "", org.Name)
	assert.Equal(t, externalID, org.ExternalID)

	return org
}

// GetOrg makes org with a random ExternalID and user for testing
func GetOrg(t *testing.T, db db.DB) (*users.User, *users.Organization) {
	user := GetUser(t, db)
	org := CreateOrgForUser(t, db, user)
	return user, org
}
