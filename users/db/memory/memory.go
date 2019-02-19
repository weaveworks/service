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

var permissions = map[string]*users.Permission{
	"team.member.invite":           {ID: "team.member.invite", Name: "Team.member.invite", Description: "derp"},
	"instance.delete":              {ID: "instance.delete", Name: "Instance.delete", Description: "derp"},
	"instance.billing.update":      {ID: "instance.billing.update", Name: "Instance.billing.update", Description: "derp"},
	"alert.settings.update":        {ID: "alert.settings.update", Name: "Alert.settings.update", Description: "derp"},
	"team.member.update":           {ID: "team.member.update", Name: "Team.member.update", Description: "derp"},
	"team.member.remove":           {ID: "team.member.remove", Name: "Team.member.remove", Description: "derp"},
	"team.members.view":            {ID: "team.members.view", Name: "Team.members.view", Description: "derp"},
	"instance.transfer":            {ID: "instance.transfer", Name: "Instance.transfer", Description: "derp"},
	"notebook.create":              {ID: "notebook.create", Name: "Notebook.create", Description: "derp"},
	"notebook.update":              {ID: "notebook.update", Name: "Notebook.update", Description: "derp"},
	"notebook.delete":              {ID: "notebook.delete", Name: "Notebook.delete", Description: "derp"},
	"scope.host.exec":              {ID: "scope.host.exec", Name: "Scope.host.exec", Description: "derp"},
	"scope.container.exec":         {ID: "scope.container.exec", Name: "Scope.container.exec", Description: "derp"},
	"scope.container.attach.out":   {ID: "scope.container.attach.out", Name: "Scope.container.attach.out", Description: "derp"},
	"scope.replicas.update":        {ID: "scope.replicas.update", Name: "Scope.replicas.update", Description: "derp"},
	"scope.pod.delete":             {ID: "scope.pod.delete", Name: "Scope.pod.delete", Description: "derp"},
	"flux.image.deploy":            {ID: "flux.image.deploy", Name: "Flux.image.deploy", Description: "derp"},
	"flux.policy.update":           {ID: "flux.policy.update", Name: "Flux.policy.update", Description: "derp"},
	"notification.settings.update": {ID: "notification.settings.update", Name: "Notification.settings.update", Description: "derp"},
	"instance.token.view":          {ID: "instance.token.view", Name: "Instance.token.view", Description: "derp"},
	"instance.webhook.create":      {ID: "instance.webhook.create", Name: "Instance.webhook.create", Description: "derp"},
	"instance.webhook.delete":      {ID: "instance.webhook.delete", Name: "Instance.webhook.delete", Description: "derp"},
}

// New creates a new in-memory database
func New(_, _ string, passwordHashingCost int) (*DB, error) {
	rolesPermissions := map[string][]string{
		"admin":  {},
		"editor": {},
		"viewer": {},
	}

	for roleID := range permissions {
		rolesPermissions["admin"] = append(rolesPermissions["admin"], roleID)
	}

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
		permissions:         permissions,
		rolesPermissions:    rolesPermissions,
		webhooks:            make(map[string][]*users.Webhook),
		passwordHashingCost: passwordHashingCost,
	}, nil
}

// Close finishes using the db. Noop.
func (d *DB) Close(_ context.Context) error {
	return nil
}
