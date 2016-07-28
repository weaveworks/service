package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Storage_RemoveOtherUsersAccess(t *testing.T) {
	setup(t)
	defer cleanup(t)

	_, org := getOrg(t)
	otherUser := getApprovedUser(t)
	otherUser, err := storage.InviteUser(otherUser.Email, org.ExternalID)
	require.NoError(t, err)
	require.Len(t, otherUser.Organizations, 1)

	orgUsers, err := storage.ListOrganizationUsers(org.ExternalID)
	require.NoError(t, err)
	require.Len(t, orgUsers, 2)

	err = storage.RemoveUserFromOrganization(org.ExternalID, otherUser.Email)
	require.NoError(t, err)

	orgUsers, err = storage.ListOrganizationUsers(org.ExternalID)
	require.NoError(t, err)
	require.Len(t, orgUsers, 1)
}
