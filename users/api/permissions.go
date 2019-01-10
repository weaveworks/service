package api

import (
	"context"

	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/users/db"
)

func hasPermission(ctx context.Context, d db.DB, userID, teamID, permissionID string) (bool, error) {
	// Get all team permissions for the user
	role, err := d.GetUserRoleInTeam(ctx, userID, teamID)
	if err != nil {
		return false, err
	}
	permissions, err := d.ListPermissionsForRoleID(ctx, role.ID)
	if err != nil {
		return false, err
	}

	// Check if the given permission is in the list
	for _, permission := range permissions {
		if permission.ID == permissionID {
			return true, nil
		}
	}
	return false, nil
}

// HasTeamMemberPermissionTo checks whether the user has a specific permission within the team.
func HasTeamMemberPermissionTo(ctx context.Context, d db.DB, userID, teamID, permissionID string) (bool, error) {
	// Get all team organizations
	orgs, err := d.ListOrganizationsInTeam(ctx, teamID)
	if err != nil {
		return false, err
	}

	// Assume team has the feature flag if any of its organizations have it.
	hasFeatureFlag := false
	for _, org := range orgs {
		if org.HasFeatureFlag(featureflag.Permissions) {
			hasFeatureFlag = true
		}
	}
	// If the permissions are not enabled, everything is allowed.
	if !hasFeatureFlag {
		return true, nil
	}

	return hasPermission(ctx, d, userID, teamID, permissionID)
}

// HasOrgMemberPermissionTo checks whether the user has a specific permission within the organization.
func HasOrgMemberPermissionTo(ctx context.Context, d db.DB, userID, orgExternalID, permissionID string) (bool, error) {
	// Find the organization from its external ID.
	org, err := d.FindOrganizationByID(ctx, orgExternalID)
	if err != nil {
		return false, err
	}

	// If the permissions are not enabled, everything is allowed.
	if !org.HasFeatureFlag(featureflag.Permissions) {
		return true, nil
	}

	return hasPermission(ctx, d, userID, org.TeamID, permissionID)
}
