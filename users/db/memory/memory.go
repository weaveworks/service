package memory

import (
	"context"
	"sync"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

// DB is an in-memory database for testing, and local development
type DB struct {
	users                map[string]*users.User                // map[userID]user
	organizations        map[string]*users.Organization        // map[id]Organization
	deletedOrganizations map[string]*users.Organization        // map[id]Organization
	logins               map[string]*login.Login               // map['provider-providerID']Login
	gcpAccounts          map[string]*users.GoogleCloudPlatform // map[externalAccountID]GCP
	teams                map[string]*users.Team                // map[id]team
	teamMemberships      map[string]map[string]string          // map[userID][teamID]roleID
	roles                map[string]*users.Role                // map[id]role
	permissions          map[string]*users.Permission          // map[id]permission
	rolesPermissions     map[string][]string                   // map[roleID][]permissionID
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
		logins:               make(map[string]*login.Login),
		gcpAccounts:          make(map[string]*users.GoogleCloudPlatform),
		teams:                make(map[string]*users.Team),
		teamMemberships:      make(map[string]map[string]string),
		roles: map[string]*users.Role{
			"admin":  {ID: "admin", Name: "Admin"},
			"editor": {ID: "editor", Name: "Editor"},
			"viewer": {ID: "viewer", Name: "Viewer"},
		},
		permissions:         make(map[string]*users.Permission),
		rolesPermissions:    make(map[string][]string),
		webhooks:            make(map[string][]*users.Webhook),
		passwordHashingCost: passwordHashingCost,
	}, nil
}

// Close finishes using the db. Noop.
func (d *DB) Close(_ context.Context) error {
	return nil
}
