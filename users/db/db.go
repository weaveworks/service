package db

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/service/common"
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
	CreateUser(ctx context.Context, email string) (*users.User, error)

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

	// Remove a user from an organization. If they do not exist (ctx context.Context, or are not a member of the org), return success.
	RemoveUserFromOrganization(ctx context.Context, orgExternalID, email string) error

	ListUsers(ctx context.Context, f filter.User) ([]*users.User, error)
	ListOrganizations(ctx context.Context, f filter.Organization) ([]*users.Organization, error)
	ListOrganizationUsers(ctx context.Context, orgExternalID string) ([]*users.User, error)

	// ListOrganizationsForUserIDs lists all organizations these users have
	// access to.
	ListOrganizationsForUserIDs(ctx context.Context, userIDs ...string) ([]*users.Organization, error)

	// ListLoginsForUserIDs lists all the logins associated with these users
	ListLoginsForUserIDs(ctx context.Context, userIDs ...string) ([]*login.Login, error)

	// Set the admin flag of a user
	SetUserAdmin(ctx context.Context, id string, value bool) error

	// Update the user's login token. Setting the token to "" should disable the
	// user's token.
	SetUserToken(ctx context.Context, id, token string) error

	// Update the user's first login timestamp. Should be called the first time a user logs in (ctx context.Context, i.e. if FirstLoginAt.IsZero())
	SetUserFirstLoginAt(ctx context.Context, id string) error

	// GenerateOrganizationExternalID generates a new, available organization ExternalID
	GenerateOrganizationExternalID(ctx context.Context) (string, error)

	// Create a new organization owned by the user. ExternalID and name cannot be blank.
	// ExternalID must match the ExternalID regex.  If token is blank, a random one will
	// be chosen.
	CreateOrganization(ctx context.Context, ownerID, externalID, name, token string) (*users.Organization, error)
	FindOrganizationByProbeToken(ctx context.Context, probeToken string) (*users.Organization, error)
	FindOrganizationByID(ctx context.Context, externalID string) (*users.Organization, error)
	FindOrganizationByGCPAccountID(ctx context.Context, gcpAccountID string) (*users.Organization, error)
	FindOrganizationByInternalID(ctx context.Context, internalID string) (*users.Organization, error)
	UpdateOrganization(ctx context.Context, externalID string, update users.OrgWriteView) error
	OrganizationExists(ctx context.Context, externalID string) (bool, error)
	ExternalIDUsed(ctx context.Context, externalID string) (bool, error)
	GetOrganizationName(ctx context.Context, externalID string) (string, error)
	DeleteOrganization(ctx context.Context, externalID string) error
	AddFeatureFlag(ctx context.Context, externalID string, featureFlag string) error
	SetFeatureFlags(ctx context.Context, externalID string, featureFlags []string) error
	SetOrganizationRefuseDataAccess(ctx context.Context, externalID string, value bool) error
	SetOrganizationRefuseDataUpload(ctx context.Context, externalID string, value bool) error
	SetOrganizationFirstSeenConnectedAt(ctx context.Context, externalID string, value *time.Time) error
	SetOrganizationZuoraAccount(ctx context.Context, externalID, number string, createdAt *time.Time) error

	GetGCP(ctx context.Context, accountID string) (*users.GoogleCloudPlatform, error)
	CreateGCP(ctx context.Context, accountID, consumerID, subscriptionName, subscriptionLevel string) (*users.GoogleCloudPlatform, error)
	UpdateGCP(ctx context.Context, accountID, consumerID, subscriptionName, subscriptionLevel string, active bool) error
	SetOrganizationGCP(ctx context.Context, externalID, gcpID string) error

	ListMemberships(ctx context.Context) ([]users.Membership, error)

	Close(ctx context.Context) error
}

// MustNew creates a new database from the URI, or panics.
func MustNew(databaseURI, migrationsDir string) DB {
	u, err := url.Parse(databaseURI)
	if err != nil {
		log.Fatal(err)
	}
	var d DB
	switch u.Scheme {
	case "memory":
		d, err = memory.New(databaseURI, migrationsDir, PasswordHashingCost)
	case "postgres":
		d, err = postgres.New(databaseURI, migrationsDir, PasswordHashingCost)
	default:
		log.Fatalf("Unknown database type: %s", u.Scheme)
	}
	if err != nil {
		log.Fatal(err)
	}
	return traced{timed{d, common.DatabaseRequestDuration}}
}
