package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/tokens"
)

// ListOrganizationWebhooks lists all webhooks for an organization
func (d *DB) ListOrganizationWebhooks(ctx context.Context, orgExternalID string) ([]*users.Webhook, error) {
	return d.webhooks[orgExternalID], nil
}

// CreateOrganizationWebhook creates a webhook given an org ID and an integration type
func (d *DB) CreateOrganizationWebhook(ctx context.Context, orgExternalID, integrationType string) (*users.Webhook, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	secretID, err := tokens.Generate()
	if err != nil {
		return nil, err
	}

	// Create secretSigningKey only if using GitHub for now.
	secretSigningKey := ""
	if integrationType == "github" {
		secretSigningKey, err = tokens.Generate()
		if err != nil {
			return nil, err
		}
	}

	o, err := d.findOrganizationByExternalID(orgExternalID)
	if err != nil {
		return nil, err
	}

	w := &users.Webhook{
		ID:               fmt.Sprint(len(d.users)),
		OrganizationID:   o.ID,
		IntegrationType:  integrationType,
		SecretID:         secretID,
		SecretSigningKey: secretSigningKey,
		CreatedAt:        time.Now().UTC(),
	}
	d.webhooks[orgExternalID] = append(d.webhooks[orgExternalID], w)

	return w, nil
}

// DeleteOrganizationWebhook deletes a webhook given it's ID
func (d *DB) DeleteOrganizationWebhook(ctx context.Context, orgExternalID, secretID string) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	orgWebhooks := d.webhooks[orgExternalID]
	for i, w := range orgWebhooks {
		if w.SecretID == secretID {
			d.webhooks[orgExternalID] = append(orgWebhooks[:i], orgWebhooks[i+1:]...)
			break
		}
	}
	return nil
}

// FindOrganizationWebhookBySecretID returns a webhook based on it's secretID
func (d *DB) FindOrganizationWebhookBySecretID(ctx context.Context, secretID string) (*users.Webhook, error) {
	for _, ws := range d.webhooks {
		for _, w := range ws {
			if w.SecretID == secretID {
				return w, nil
			}
		}
	}
	return nil, fmt.Errorf("webhook not found")
}
