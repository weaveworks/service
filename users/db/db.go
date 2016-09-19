package db

import (
	"encoding/json"
	"net/url"
	"sync"

	"github.com/Sirupsen/logrus"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

var (
	// PasswordHashingCost sets the difficulty we want to use when hashing
	// password. It should be high enough to be difficult, but low enough we can
	// do it.
	PasswordHashingCost = 14

	drivers     = map[string]func(string, string) (DB, error){}
	driversLock sync.Mutex
)

// DB is the interface for the database.
type DB interface {
	// Create a user. The driver should set ID to some default only when it is "".
	CreateUser(email string) (*users.User, error)

	users.FindUserByIDer
	FindUserByEmail(email string) (*users.User, error)
	FindUserByLogin(provider, id string) (*users.User, error)
	FindUserByAPIToken(token string) (*users.User, error)

	UserIsMemberOf(userID, orgExternalID string) (bool, error)

	// AddLoginToUser adds an entry denoting this user is linked to a
	// remote login. e.g. if a user logs in via github this maps our
	// account to the github account.
	// Note: Must be idempotent!
	AddLoginToUser(userID, provider, id string, session json.RawMessage) error

	// DetachLoginFromUser removes all entries an entry denoting this
	// user is linked to the remote login.
	DetachLoginFromUser(userID, provider string) error

	// Create an API Token for a user
	CreateAPIToken(userID, description string) (*users.APIToken, error)

	// Delete an API Token for a user
	DeleteAPIToken(userID, token string) error

	// Invite a user to access an existing organization.
	InviteUser(email, orgExternalID string) (*users.User, bool, error)

	// Remove a user from an organization. If they do not exist (or are not a member of the org), return success.
	RemoveUserFromOrganization(orgExternalID, email string) error

	ListUsers() ([]*users.User, error)
	ListOrganizations() ([]*users.Organization, error)
	ListOrganizationUsers(orgExternalID string) ([]*users.User, error)

	// ListOrganizationsForUserIDs lists all organizations these users have
	// access to.
	ListOrganizationsForUserIDs(userIDs ...string) ([]*users.Organization, error)

	// ListLoginsForUserIDs lists all the logins associated with these users
	ListLoginsForUserIDs(userIDs ...string) ([]*login.Login, error)

	// ListAPITokensForUserIDs lists all the api tokens associated with these
	// users
	ListAPITokensForUserIDs(userIDs ...string) ([]*users.APIToken, error)

	// Approve the user for access. Should generate them a new organization.
	ApproveUser(id string) (*users.User, error)

	// Set the admin flag of a user
	SetUserAdmin(id string, value bool) error

	// Update the user's login token. Setting the token to "" should disable the
	// user's token.
	SetUserToken(id, token string) error

	// Update the user's first login timestamp. Should be called the first time a user logs in (i.e. if FirstLoginAt.IsZero())
	SetUserFirstLoginAt(id string) error

	// GenerateOrganizationExternalID generates a new, available organization ExternalID
	GenerateOrganizationExternalID() (string, error)

	// Create a new organization owned by the user. ExternalID and name cannot be blank.
	// ExternalID must match the ExternalID regex.
	CreateOrganization(ownerID, externalID, name string) (*users.Organization, error)
	FindOrganizationByProbeToken(probeToken string) (*users.Organization, error)
	RenameOrganization(externalID, newName string) error
	OrganizationExists(externalID string) (bool, error)
	GetOrganizationName(externalID string) (string, error)
	DeleteOrganization(externalID string) error
	AddFeatureFlag(externalID string, featureFlag string) error

	Close() error
}

// Truncater is used in testing, but is not part of the normal db interface
type Truncater interface {
	Truncate() error
}

// Register is what drivers should call to make themselves available.
func Register(scheme string, f func(string, string) (DB, error)) {
	driversLock.Lock()
	drivers[scheme] = f
	driversLock.Unlock()
}

// MustNew creates a new database from the URI, or panics.
func MustNew(databaseURI, migrationsDir string) DB {
	u, err := url.Parse(databaseURI)
	if err != nil {
		logrus.Fatal(err)
	}
	driversLock.Lock()
	driver, ok := drivers[u.Scheme]
	if !ok {
		known := []string{}
		for scheme := range drivers {
			known = append(known, scheme)
		}
		logrus.Fatalf("Unknown database type: %s, have %q", u.Scheme, known)
	}
	defer driversLock.Unlock()

	db, err := driver(databaseURI, migrationsDir)
	if err != nil {
		logrus.Fatal(err)
	}
	return traced{timed{db, users.DatabaseRequestDuration}}
}
