package memory

import (
	"sync"

	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

// DB is an in-memory database for testing, and local development
type DB struct {
	users               map[string]*users.User
	organizations       map[string]*users.Organization
	memberships         map[string][]string
	logins              map[string]*login.Login
	passwordHashingCost int
	mtx                 sync.Mutex
}

// New creates a new in-memory database
func New(_, _ string, passwordHashingCost int) (*DB, error) {
	return &DB{
		users:               make(map[string]*users.User),
		organizations:       make(map[string]*users.Organization),
		memberships:         make(map[string][]string),
		logins:              make(map[string]*login.Login),
		passwordHashingCost: passwordHashingCost,
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
