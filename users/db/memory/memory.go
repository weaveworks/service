package memory

import (
	"context"
	"sync"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

// DB is an in-memory database for testing, and local development
type DB struct {
	users                map[string]*users.User // map[userID]user
	organizations        map[string]*users.Organization
	deletedOrganizations map[string]*users.Organization
	memberships          map[string][]string // map[orgID][]userID
	logins               map[string]*login.Login
	gcpAccounts          map[string]*users.GoogleCloudPlatform // map[externalAccountID]GCP
	teams                map[string]*users.Team                // map[id]team
	teamMemberships      map[string][]string                   // map[userID][]teamID
	webhooks             map[string][]*users.Webhook           // map[externalOrgID]webhook
	passwordHashingCost  int
	mtx                  sync.Mutex
}

// New creates a new in-memory database
func New(_, _ string, passwordHashingCost int) (*DB, error) {
	return &DB{
		users:                make(map[string]*users.User),
		organizations:        make(map[string]*users.Organization),
		deletedOrganizations: make(map[string]*users.Organization),
		memberships:          make(map[string][]string),
		logins:               make(map[string]*login.Login),
		gcpAccounts:          make(map[string]*users.GoogleCloudPlatform),
		teams:                make(map[string]*users.Team),
		teamMemberships:      make(map[string][]string),
		webhooks:             make(map[string][]*users.Webhook),
		passwordHashingCost:  passwordHashingCost,
	}, nil
}

// ListMemberships lists memberships list memberships
func (d *DB) ListMemberships(_ context.Context) ([]users.Membership, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	memberships := []users.Membership{}
	for orgID, userIDs := range d.memberships {
		for _, userID := range userIDs {
			memberships = append(memberships, users.Membership{
				UserID:         userID,
				OrganizationID: orgID,
			})
		}
	}
	return memberships, nil
}

// Close finishes using the db. Noop.
func (d *DB) Close(_ context.Context) error {
	return nil
}
