package dbtest

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

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
