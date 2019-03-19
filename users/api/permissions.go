package api

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
)

func requirePermission(ctx context.Context, d db.DB, userID, teamID, permissionID string) error {
	// Get all team permissions for the user
	role, err := d.GetUserRoleInTeam(ctx, userID, teamID)
	if err == users.ErrNotFound {
		// If user is not part of the team, forbid any actions on it
		return users.ErrForbidden
	} else if err != nil {
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

	log.Errorf("Permission denied (userID: %s, teamID: %s, permissionID: %s)", userID, teamID, permissionID)

	return users.ErrForbidden
}

// RequireTeamMemberPermissionTo requires team member permission for a specific action (and returns an error if denied).
func RequireTeamMemberPermissionTo(ctx context.Context, d db.DB, userID, teamExternalID, permissionID string) error {
	// Find the team from its external ID.
	team, err := d.FindTeamByExternalID(ctx, teamExternalID)
	if err != nil {
		return err
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
	return requirePermission(ctx, d, userID, org.TeamID, permissionID)
}
