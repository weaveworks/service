package dbtest

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
)

// GetUser makes a randomly named user
func GetUser(t *testing.T, db db.DB) *users.User {
	email := fmt.Sprintf("%d@weave.works", rand.Int63())
	user, err := db.CreateUser(email)
	require.NoError(t, err)

	return user
}

// CreateOrgForUser creates a new random organization for this user
func CreateOrgForUser(t *testing.T, db db.DB, u *users.User) *users.Organization {
	externalID, err := db.GenerateOrganizationExternalID()
	require.NoError(t, err)

	org, err := db.CreateOrganization(u.ID, externalID, externalID, "")
	require.NoError(t, err)

	assert.NotEqual(t, "", org.ID)
	assert.NotEqual(t, "", org.ExternalID)
	assert.Equal(t, org.ExternalID, org.Name)

	return org
}

// GetOrg makes org with a random ExternalID and user for testing
func GetOrg(t *testing.T, db db.DB) (*users.User, *users.Organization) {
	user := GetUser(t, db)
	org := CreateOrgForUser(t, db, user)
	return user, org
}
