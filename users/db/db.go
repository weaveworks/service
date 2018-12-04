package db

import (
	"context"
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/db/memory"
	"github.com/weaveworks/service/users/db/postgres"
	"github.com/weaveworks/service/users/login"
)

var (
	// PasswordHashingCost sets the difficulty we want to use when hashing
	// password. It should be high enough to be difficult, but low enough we can
	// do it.
	PasswordHashingCost = 14
)

// DB is the interface for the database.
type DB interface {
	// Create a user. The driver should set ID to some default only when it is "".
	CreateUser(ctx context.Context, email string, details *users.UserUpdate) (*users.User, error)
	UpdateUser(ctx context.Context, userID string, update *users.UserUpdate) (*users.User, error)
	DeleteUser(ctx context.Context, userID string) error

	users.FindUserByIDer
	FindUserByEmail(ctx context.Context, email string) (*users.User, error)
	FindUserByLogin(ctx context.Context, provider, id string) (*users.User, error)

	UserIsMemberOf(ctx context.Context, userID, orgExternalID string) (bool, error)

	// AddLoginToUser adds an entry denoting this user is linked to a
	// remote login. e.g. if a user logs in via github this maps our
	// account to the github account.
	// Note: Must be idempotent!
	AddLoginToUser(ctx context.Context, userID, provider, id string, session json.RawMessage) error

	// DetachLoginFromUser removes all entries an entry denoting this
	// user is linked to the remote login.
	DetachLoginFromUser(ctx context.Context, userID, provider string) error

	// Invite a user to access an existing organization.
	InviteUser(ctx context.Context, email, orgExternalID string) (*users.User, bool, error)

	// Remove a user from an organization. If they do not exist (or are not a member of the org), return success.
	RemoveUserFromOrganization(ctx context.Context, orgExternalID, email string) error

	// List users who match a filter
	// NB: page 0 will return all matches. Use page >= 1 for paginated responses
	ListUsers(ctx context.Context, f filter.User, page uint64) ([]*users.User, error)
	ListOrganizations(ctx context.Context, f filter.Organization, page uint64) ([]*users.Organization, error)
	ListAllOrganizations(ctx context.Context, f filter.Organization, page uint64) ([]*users.Organization, error)
	ListOrganizationUsers(ctx context.Context, orgExternalID string, includeDeletedOrgs bool) ([]*users.User, error)

	// ListOrganizationsForUserIDs lists all organizations these users have
	// access to.
	ListOrganizationsForUserIDs(ctx context.Context, userIDs ...string) ([]*users.Organization, error)
	ListAllOrganizationsForUserIDs(ctx context.Context, userIDs ...string) ([]*users.Organization, error)

	// ListLoginsForUserIDs lists all the logins associated with these users
	ListLoginsForUserIDs(ctx context.Context, userIDs ...string) ([]*login.Login, error)

	// Set the admin flag of a user
	SetUserAdmin(ctx context.Context, id string, value bool) error

	// Update the user's login token. Setting the token to "" should disable the
	// user's token.
	SetUserToken(ctx context.Context, id, token string) error

	// Update the user's last login timestamp. If it is the user's first login, also set the user's first login timestamp
	SetUserLastLoginAt(ctx context.Context, id string) error

	// GenerateOrganizationExternalID generates a new, available organization ExternalID
	GenerateOrganizationExternalID(ctx context.Context) (string, error)

	// Create a new organization owned by the user. ExternalID and name cannot be blank.
	// ExternalID must match the ExternalID regex.  If token is blank, a random one will
	// be chosen.
	CreateOrganization(ctx context.Context, ownerID, externalID, name, token, teamID string, trialExpiresAt time.Time) (*users.Organization, error)
	FindUncleanedOrgIDs(ctx context.Context) ([]string, error)
	FindOrganizationByProbeToken(ctx context.Context, probeToken string) (*users.Organization, error)
	FindOrganizationByID(ctx context.Context, externalID string) (*users.Organization, error)
	FindOrganizationByGCPExternalAccountID(ctx context.Context, externalAccountID string) (*users.Organization, error)
	FindOrganizationByInternalID(ctx context.Context, internalID string) (*users.Organization, error)
	UpdateOrganization(ctx context.Context, externalID string, update users.OrgWriteView) (*users.Organization, error)
	OrganizationExists(ctx context.Context, externalID string) (bool, error)
	ExternalIDUsed(ctx context.Context, externalID string) (bool, error)
	GetOrganizationName(ctx context.Context, externalID string) (string, error)
	DeleteOrganization(ctx context.Context, externalID string) error
	AddFeatureFlag(ctx context.Context, externalID string, featureFlag string) error
	SetOrganizationCleanup(ctx context.Context, internalID string, value bool) error
	SetFeatureFlags(ctx context.Context, externalID string, featureFlags []string) error
	SetOrganizationRefuseDataAccess(ctx context.Context, externalID string, value bool) error
	SetOrganizationRefuseDataUpload(ctx context.Context, externalID string, value bool) error
	SetOrganizationRefuseDataReason(ctx context.Context, externalID string, reason string) error
	SetOrganizationFirstSeenConnectedAt(ctx context.Context, externalID string, value *time.Time) error
	SetOrganizationFirstSeenFluxConnectedAt(ctx context.Context, externalID string, value *time.Time) error
	SetOrganizationFirstSeenNetConnectedAt(ctx context.Context, externalID string, value *time.Time) error
	SetOrganizationFirstSeenPromConnectedAt(ctx context.Context, externalID string, value *time.Time) error
	SetOrganizationFirstSeenScopeConnectedAt(ctx context.Context, externalID string, value *time.Time) error
	SetOrganizationZuoraAccount(ctx context.Context, externalID, number string, createdAt *time.Time) error
	SetOrganizationPlatformVersion(ctx context.Context, externalID, platformVersion string) error
	SetLastSentWeeklyReportAt(ctx context.Context, externalID string, sentAt *time.Time) error
	// MoveOrganizationToTeam updates the team of the organization. It does *not* check team permissions.
	MoveOrganizationToTeam(ctx context.Context, externalID, teamExternalID, teamName, userID string) error

	// CreateOrganizationWithGCP creates an organization with an inactive GCP account attached to it.
	CreateOrganizationWithGCP(ctx context.Context, ownerID, externalAccountID string, trialExpiresAt time.Time) (*users.Organization, error)
	// FindGCP returns the Google Cloud Platform subscription for the given account.
	FindGCP(ctx context.Context, externalAccountID string) (*users.GoogleCloudPlatform, error)
	// UpdateGCP Update a Google Cloud Platform entry. This marks the account as activated.
	UpdateGCP(ctx context.Context, externalAccountID, consumerID, subscriptionName, subscriptionLevel, subscriptionStatus string) error
	// SetOrganizationGCP attaches a Google Cloud Platform subscription to an organization.
	// It also enables the billing feature flag and sets platform/env.
	SetOrganizationGCP(ctx context.Context, externalID, externalAccountID string) error

	ListMemberships(ctx context.Context) ([]users.Membership, error)

	ListTeamsForUserID(ctx context.Context, userID string) ([]*users.Team, error)
	ListTeamUsers(ctx context.Context, teamID string) ([]*users.User, error)
	ListPermissionsForRoleID(ctx context.Context, roleID string) ([]*users.Permission, error)
	GetUserRoleInTeam(ctx context.Context, userID string, teamID string) (*users.Role, error)
	FindTeamByExternalID(ctx context.Context, externalID string) (*users.Team, error)
	CreateTeam(_ context.Context, name string) (*users.Team, error)
	AddUserToTeam(_ context.Context, userID, teamID string) error
	DeleteTeam(ctx context.Context, teamID string) error
	CreateOrganizationWithTeam(ctx context.Context, ownerID, externalID, name, token, teamExternalID, teamName string, trialExpiresAt time.Time) (*users.Organization, error)

	// Webhooks
	ListOrganizationWebhooks(ctx context.Context, orgExternalID string) ([]*users.Webhook, error)
	CreateOrganizationWebhook(ctx context.Context, orgExternalID, integrationType string) (*users.Webhook, error)
	DeleteOrganizationWebhook(ctx context.Context, orgExternalID, secretID string) error
	FindOrganizationWebhookBySecretID(ctx context.Context, secretID string) (*users.Webhook, error)
	SetOrganizationWebhookFirstSeenAt(ctx context.Context, secretID string) (*time.Time, error)

	// GetSummary exports a summary of the DB.
	// WARNING: this is a relatively expensive query, and basically exports the entire DB.
	GetSummary(ctx context.Context) ([]*users.SummaryEntry, error)

	Close(ctx context.Context) error
}

// MustNew creates a new database from the URI, or panics.
func MustNew(cfg dbconfig.Config) DB {
	scheme, dataSourceName, migrationsDir, err := cfg.Parameters()
	if err != nil {
		log.Fatal(err)
	}
	var d DB
	switch scheme {
	case "memory":
		d, err = memory.New(dataSourceName, migrationsDir, PasswordHashingCost)
	case "postgres":
		d, err = postgres.New(dataSourceName, migrationsDir, PasswordHashingCost)
	default:
		log.Fatalf("Unknown database type: %s", scheme)
	}
	if err != nil {
		log.Fatal(err)
	}
	return traced{timed{d, common.DatabaseRequestDuration}}
}
