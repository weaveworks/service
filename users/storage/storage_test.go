package storage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	storagetest "github.com/weaveworks/service/users/storage/test"
)

func Test_Storage_RemoveOtherUsersAccess(t *testing.T) {
	db := storagetest.Setup(t)
	defer storagetest.Cleanup(t, db)

	_, org := storagetest.GetOrg(t, db)
	otherUser := storagetest.GetApprovedUser(t, db)
	otherUser, _, err := db.InviteUser(otherUser.Email, org.ExternalID)
	require.NoError(t, err)
	require.Len(t, otherUser.Organizations, 1)

	orgUsers, err := db.ListOrganizationUsers(org.ExternalID)
	require.NoError(t, err)
	require.Len(t, orgUsers, 2)

	err = db.RemoveUserFromOrganization(org.ExternalID, otherUser.Email)
	require.NoError(t, err)

	orgUsers, err = db.ListOrganizationUsers(org.ExternalID)
	require.NoError(t, err)
	require.Len(t, orgUsers, 1)
}

func Test_Storage_AddFeatureFlag(t *testing.T) {
	db := storagetest.Setup(t)
	defer storagetest.Cleanup(t, db)

	_, org := storagetest.GetOrg(t, db)
	err := db.AddFeatureFlag(org.ExternalID, "supercow")
	require.NoError(t, err)

	org, err = db.FindOrganizationByProbeToken(org.ProbeToken)
	require.NoError(t, err)
	require.Equal(t, org.FeatureFlags, []string{"supercow"})
}
