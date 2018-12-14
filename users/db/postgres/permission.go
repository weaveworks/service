package postgres

import (
	"context"
	"database/sql"

	"github.com/Masterminds/squirrel"

	"github.com/weaveworks/service/users"
)

// ListPermissionsForRoleID lists all permissions for a role
func (d DB) ListPermissionsForRoleID(ctx context.Context, roleID string) ([]*users.Permission, error) {
	rows, err := d.permissionsQuery().
		Join("roles_permissions on (roles_permissions.permission_id = permissions.id)").
		Where("roles_permissions.deleted_at is null").
		Where(squirrel.Eq{
			"roles_permissions.role_id": roleID,
		}).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanPermissions(rows)
}

func (d DB) permissionsQuery() squirrel.SelectBuilder {
	return d.Select(`
		permissions.id,
		permissions.name,
		permissions.description
	`).
		From("permissions").
		Where("permissions.deleted_at is null").
		OrderBy("permissions.created_at")
}

func (d DB) scanPermissions(rows *sql.Rows) ([]*users.Permission, error) {
	permissions := []*users.Permission{}
	for rows.Next() {
		permission, err := d.scanPermission(rows)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return permissions, nil
}

func (d DB) scanPermission(row squirrel.RowScanner) (*users.Permission, error) {
	p := &users.Permission{}
	if err := row.Scan(&p.ID, &p.Name, &p.Description); err != nil {
		return nil, err
	}
	return p, nil
}
