package memory

import (
	"context"

	"github.com/weaveworks/service/users"
)

// ListRoles lists all user roles
func (d *DB) ListRoles(_ context.Context) ([]*users.Role, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	var roles []*users.Role
	for _, role := range d.roles {
		roles = append(roles, role)
	}

	return roles, nil
}
