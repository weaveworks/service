package postgres

import (
	"context"
	"database/sql"

	"github.com/Masterminds/squirrel"

	"github.com/weaveworks/service/users"
)

func (d DB) rolesQuery() squirrel.SelectBuilder {
	return d.Select(`
		roles.id,
		roles.name,
		roles.description
	`).
		From("roles").
		Where("roles.deleted_at is null").
		OrderBy("roles.created_at")
}

func (d DB) scanRoles(rows *sql.Rows) ([]*users.Role, error) {
	var roles []*users.Role
	for rows.Next() {
		role, err := d.scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return roles, nil
}

func (d DB) scanRole(row squirrel.RowScanner) (*users.Role, error) {
	r := &users.Role{}
	if err := row.Scan(&r.ID, &r.Name, &r.Description); err != nil {
		return nil, err
	}
	return r, nil
}

// ListRoles returns all user roles
func (d DB) ListRoles(ctx context.Context) ([]*users.Role, error) {
	query := d.rolesQuery()
	rows, err := query.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanRoles(rows)
}
