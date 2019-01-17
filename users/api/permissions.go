package api

import (
	"context"
	"fmt"

	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/users/db"
)

func requirePermission(ctx context.Context, d db.DB, userID, teamID, permissionID string) error {
	// Get all team permissions for the user
	role, err := d.GetUserRoleInTeam(ctx, userID, teamID)
	if err != nil {
		return err
	}
	permissions, err := d.ListPermissionsForRoleID(ctx, role.ID)
	if err != nil {
		return err
	}

	// Check if the given permission is in the list
	for _, permission := range permissions {
		if permission.ID == permissionID {
			return nil
		}
	}
	return fmt.Errorf("Permission denied (userID: %s, teamID: %s, permissionID: %s)", userID, teamID, permissionID)
}

// RequireTeamMemberPermissionTo requires team member permission for a specific action (and returns an error if denied).
func RequireTeamMemberPermissionTo(ctx context.Context, d db.DB, userID, teamExternalID, permissionID string) error {
	// Get all team organizations
	team, err := d.FindTeamByExternalID(ctx, teamExternalID)
	if err != nil {
		return err
	}
	orgs, err := d.ListOrganizationsInTeam(ctx, team.ID)
	if err != nil {
		return err
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
		return nil
	}

	return requirePermission(ctx, d, userID, team.ID, permissionID)
}

// RequireOrgMemberPermissionTo requires instance member permission for a specific action (and returns an error if denied).
func RequireOrgMemberPermissionTo(ctx context.Context, d db.DB, userID, orgExternalID, permissionID string) error {
	// Find the organization from its external ID.
	org, err := d.FindOrganizationByID(ctx, orgExternalID)
	if err != nil {
		return err
	}

	// If the permissions are not enabled, everything is allowed.
	if !org.HasFeatureFlag(featureflag.Permissions) {
		return nil
	}

	return requirePermission(ctx, d, userID, org.TeamID, permissionID)
}
