package postgres

import (
	"context"
	"database/sql"

	"github.com/Masterminds/squirrel"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/tokens"
)

// ListOrganizationWebhooks lists all webhooks for an organization
func (d DB) ListOrganizationWebhooks(ctx context.Context, orgExternalID string) ([]*users.Webhook, error) {
	rows, err := d.webhooksQuery().
		Join("organizations ON (webhooks.organization_id = organizations.id)").
		Where("organizations.external_id = ?", orgExternalID).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	return d.scanWebhooks(rows)
}

// CreateOrganizationWebhook creates a webhook given an org ID and an integration type
func (d DB) CreateOrganizationWebhook(ctx context.Context, orgExternalID, integrationType string) (*users.Webhook, error) {
	secretID, err := tokens.Generate()
	if err != nil {
		return nil, err
	}
	secretSigningKey, err := tokens.Generate()
	if err != nil {
		return nil, err
	}

	org, err := d.FindOrganizationByID(ctx, orgExternalID)
	if err != nil {
		return nil, err
	}

	w := &users.Webhook{
		OrganizationID:   org.ID,
		IntegrationType:  integrationType,
		SecretID:         secretID,
		SecretSigningKey: secretSigningKey,
	}
	err = d.Insert("webhooks").
		Columns("organization_id", "integration_type", "secret_id", "secret_signing_key").
		Values(w.OrganizationID, w.IntegrationType, w.SecretID, w.SecretSigningKey).
		Suffix("RETURNING id, created_at").
		QueryRowContext(ctx).
		Scan(&w.ID, &w.CreatedAt)

	switch {
	case err == sql.ErrNoRows:
		return nil, users.ErrNotFound
	case err != nil:
		return nil, err
	}
	return w, nil
}

// DeleteOrganizationWebhook deletes a webhook given it's ID
func (d DB) DeleteOrganizationWebhook(ctx context.Context, orgExternalID, secretID string) error {
	// Fetch internal ID so we can do the update.
	org, err := d.FindOrganizationByID(ctx, orgExternalID)
	if err != nil {
		return err
	}

	deletedAt := d.Now()
	_, err = d.Update("webhooks").
		Set("deleted_at", deletedAt).
		Where("webhooks.organization_id = ?", string(org.ID)).
		Where("webhooks.secret_id = ?", string(secretID)).
		Where("webhooks.deleted_at is null").
		ExecContext(ctx)
	if err != nil {
		return err
	}
	return nil
}

// FindOrganizationWebhookBySecretID returns a webhook based on it's secretID
func (d DB) FindOrganizationWebhookBySecretID(ctx context.Context, orgExternalID, secretID string) (*users.Webhook, error) {
	query := d.webhooksQuery().
		Join("organizations ON (webhooks.organization_id = organizations.id)").
		Where("organizations.external_id = ?", string(orgExternalID)).
		Where("webhooks.secret_id = ?", string(secretID))
	row := query.QueryRowContext(ctx)
	return d.scanWebhook(row)
}

func (d DB) scanWebhooks(rows *sql.Rows) ([]*users.Webhook, error) {
	webhooks := []*users.Webhook{}
	for rows.Next() {
		webhook, err := d.scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, webhook)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return webhooks, nil
}

func (d DB) scanWebhook(row squirrel.RowScanner) (*users.Webhook, error) {
	w := &users.Webhook{}
	if err := row.Scan(
		&w.ID, &w.OrganizationID, &w.IntegrationType, &w.SecretID,
		&w.SecretSigningKey, &w.CreatedAt, &w.DeletedAt,
	); err != nil {
		return nil, err
	}
	return w, nil
}

func (d DB) webhooksQuery() squirrel.SelectBuilder {
	return d.Select(
		"webhooks.id",
		"webhooks.organization_id",
		"webhooks.integration_type",
		"webhooks.secret_id",
		"webhooks.secret_signing_key",
		"webhooks.created_at",
		"webhooks.deleted_at",
	).
		From("webhooks").
		Where("webhooks.deleted_at is null").
		OrderBy("webhooks.created_at ASC")
}
