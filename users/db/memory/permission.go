package memory

import (
	"context"

	"github.com/weaveworks/service/users"
)

// ListPermissionsForRoleID lists the permissions belonging to the given role
func (d *DB) ListPermissionsForRoleID(_ context.Context, roleID string) ([]*users.Permission, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	var permissions []*users.Permission
	permissionIDs, exists := d.rolesPermissions[roleID]
	if !exists {
		return permissions, nil
	}

	for _, permissionID := range permissionIDs {
		permission := d.permissions[permissionID]
		permissions = append(permissions, permission)
	}

	return permissions, nil
}
