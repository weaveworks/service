package storage

import (
	"database/sql"
	"encoding/json"
	"net/url"

	"github.com/Sirupsen/logrus"
	_ "github.com/mattes/migrate/driver/postgres" // Import the postgres migrations driver
	"github.com/mattes/migrate/migrate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/instrument"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

var (
	// PasswordHashingCost sets the difficulty we want to use when hashing
	// password. It should be high enough to be difficult, but low enough we can
	// do it.
	PasswordHashingCost = 14
)

// Database is the interface for the database.
type Database interface {
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

// MustNew creates a new database from the URI, or panics.
func MustNew(databaseURI, migrationsDir string) Database {
	u, err := url.Parse(databaseURI)
	if err != nil {
		logrus.Fatal(err)
	}
	if migrationsDir != "" {
		logrus.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(databaseURI, migrationsDir); !ok {
			for _, err := range errs {
				logrus.Error(err)
			}
			logrus.Fatal("Database migrations failed")
		}
	}
	var storage Database
	switch u.Scheme {
	case "postgres":
		db, err := sql.Open(u.Scheme, databaseURI)
		if err != nil {
			logrus.Fatal(err)
		}
		storage = newPGStorage(db)
	case "memory":
		storage = newMemoryStorage()
	default:
		logrus.Fatalf("Unknown database type: %s", u.Scheme)
	}
	return TimedDatabase{storage, users.DatabaseRequestDuration}
}

// TimedDatabase adds prometheus timings to another database implementation
type TimedDatabase struct {
	d        Database
	Duration *prometheus.HistogramVec
}

func (t TimedDatabase) errorCode(err error) string {
	switch err {
	case nil:
		return "200"
	case users.ErrNotFound:
		return "404"
	case users.ErrEmailIsTaken:
		return "400"
	case users.ErrInvalidAuthenticationData:
		return "401"
	default:
		return "500"
	}
}

func (t TimedDatabase) timeRequest(method string, f func() error) error {
	return instrument.TimeRequestHistogramStatus(method, t.Duration, t.errorCode, f)
}

// CreateUser creates a user
func (t TimedDatabase) CreateUser(email string) (u *users.User, err error) {
	t.timeRequest("CreateUser", func() error {
		u, err = t.d.CreateUser(email)
		return err
	})
	return
}

// FindUserByID finds a user by id
func (t TimedDatabase) FindUserByID(id string) (u *users.User, err error) {
	t.timeRequest("FindUserByID", func() error {
		u, err = t.d.FindUserByID(id)
		return err
	})
	return
}

// FindUserByEmail finds a user by email
func (t TimedDatabase) FindUserByEmail(email string) (u *users.User, err error) {
	t.timeRequest("FindUserByEmail", func() error {
		u, err = t.d.FindUserByEmail(email)
		return err
	})
	return
}

// FindUserByLogin finds a user by login
func (t TimedDatabase) FindUserByLogin(provider, id string) (u *users.User, err error) {
	t.timeRequest("FindUserByLogin", func() error {
		u, err = t.d.FindUserByLogin(provider, id)
		return err
	})
	return
}

// FindUserByAPIToken finds a user by api token
func (t TimedDatabase) FindUserByAPIToken(token string) (u *users.User, err error) {
	t.timeRequest("FindUserByAPIToken", func() error {
		u, err = t.d.FindUserByAPIToken(token)
		return err
	})
	return
}

// UserIsMemberOf checks if the user is a member of the organization
func (t TimedDatabase) UserIsMemberOf(userID, orgExternalID string) (b bool, err error) {
	t.timeRequest("UserIsMemberOf", func() error {
		b, err = t.d.UserIsMemberOf(userID, orgExternalID)
		return err
	})
	return
}

// AddLoginToUser adds the login to the user
func (t TimedDatabase) AddLoginToUser(userID, provider, id string, session json.RawMessage) error {
	return t.timeRequest("AddLoginToUser", func() error {
		return t.d.AddLoginToUser(userID, provider, id, session)
	})
}

// DetachLoginFromUser detaches a login from the user
func (t TimedDatabase) DetachLoginFromUser(userID, provider string) error {
	return t.timeRequest("DetachLoginFromUser", func() error {
		return t.d.DetachLoginFromUser(userID, provider)
	})
}

// CreateAPIToken creates an API Token for a user
func (t TimedDatabase) CreateAPIToken(userID, description string) (token *users.APIToken, err error) {
	t.timeRequest("CreateAPIToken", func() error {
		token, err = t.d.CreateAPIToken(userID, description)
		return err
	})
	return
}

// DeleteAPIToken deletes an API Token for a user
func (t TimedDatabase) DeleteAPIToken(userID, token string) error {
	return t.timeRequest("DeleteAPIToken", func() error {
		return t.d.DeleteAPIToken(userID, token)
	})
}

// InviteUser invites a user to join an organization
func (t TimedDatabase) InviteUser(email, orgExternalID string) (u *users.User, created bool, err error) {
	t.timeRequest("InviteUser", func() error {
		u, created, err = t.d.InviteUser(email, orgExternalID)
		return err
	})
	return
}

// RemoveUserFromOrganization removes a user from the organization
func (t TimedDatabase) RemoveUserFromOrganization(orgExternalID, email string) error {
	return t.timeRequest("RemoveUserFromOrganization", func() error {
		return t.d.RemoveUserFromOrganization(orgExternalID, email)
	})
}

// ListUsers lists users
func (t TimedDatabase) ListUsers() (us []*users.User, err error) {
	t.timeRequest("ListUsers", func() error {
		us, err = t.d.ListUsers()
		return err
	})
	return
}

// ListOrganizations lists organizations
func (t TimedDatabase) ListOrganizations() (os []*users.Organization, err error) {
	t.timeRequest("ListOrganizations", func() error {
		os, err = t.d.ListOrganizations()
		return err
	})
	return
}

// ListOrganizationUsers lists users in an organization
func (t TimedDatabase) ListOrganizationUsers(orgExternalID string) (us []*users.User, err error) {
	t.timeRequest("ListOrganizationUsers", func() error {
		us, err = t.d.ListOrganizationUsers(orgExternalID)
		return err
	})
	return
}

// ListOrganizationsForUserIDs lists the organizations these users belong to
func (t TimedDatabase) ListOrganizationsForUserIDs(userIDs ...string) (os []*users.Organization, err error) {
	t.timeRequest("ListOrganizationsForUserIDs", func() error {
		os, err = t.d.ListOrganizationsForUserIDs(userIDs...)
		return err
	})
	return
}

// ListLoginsForUserIDs lists the logins for these users
func (t TimedDatabase) ListLoginsForUserIDs(userIDs ...string) (ls []*login.Login, err error) {
	t.timeRequest("ListLoginsForUserIDs", func() error {
		ls, err = t.d.ListLoginsForUserIDs(userIDs...)
		return err
	})
	return
}

// ListAPITokensForUserIDs lists the api tokens for these users
func (t TimedDatabase) ListAPITokensForUserIDs(userIDs ...string) (ts []*users.APIToken, err error) {
	t.timeRequest("ListAPITokensForUserIDs", func() error {
		ts, err = t.d.ListAPITokensForUserIDs(userIDs...)
		return err
	})
	return
}

// ApproveUser approves a user to begin using the service
func (t TimedDatabase) ApproveUser(id string) (u *users.User, err error) {
	t.timeRequest("ApproveUser", func() error {
		u, err = t.d.ApproveUser(id)
		return err
	})
	return
}

// SetUserAdmin sets a user's admin flag
func (t TimedDatabase) SetUserAdmin(id string, value bool) error {
	return t.timeRequest("SetUserAdmin", func() error {
		return t.d.SetUserAdmin(id, value)
	})
}

// SetUserToken sets a user's login token
func (t TimedDatabase) SetUserToken(id, token string) error {
	return t.timeRequest("SetUserToken", func() error {
		return t.d.SetUserToken(id, token)
	})
}

// SetUserFirstLoginAt sets a user's first login at timestamp
func (t TimedDatabase) SetUserFirstLoginAt(id string) error {
	return t.timeRequest("SetUserFirstLoginAt", func() error {
		return t.d.SetUserFirstLoginAt(id)
	})
}

// GenerateOrganizationExternalID generates an available organization external id
func (t TimedDatabase) GenerateOrganizationExternalID() (s string, err error) {
	t.timeRequest("GenerateOrganizationExternalID", func() error {
		s, err = t.d.GenerateOrganizationExternalID()
		return err
	})
	return
}

// CreateOrganization creates a new organization
func (t TimedDatabase) CreateOrganization(ownerID, externalID, name string) (o *users.Organization, err error) {
	t.timeRequest("CreateOrganization", func() error {
		o, err = t.d.CreateOrganization(ownerID, externalID, name)
		return err
	})
	return
}

// FindOrganizationByProbeToken finds a organization from it's probe token
func (t TimedDatabase) FindOrganizationByProbeToken(probeToken string) (o *users.Organization, err error) {
	t.timeRequest("FindOrganizationByProbeToken", func() error {
		o, err = t.d.FindOrganizationByProbeToken(probeToken)
		return err
	})
	return
}

// RenameOrganization changes an organization's user-settable name
func (t TimedDatabase) RenameOrganization(externalID, name string) error {
	return t.timeRequest("RenameOrganization", func() error {
		return t.d.RenameOrganization(externalID, name)
	})
}

// OrganizationExists checks if an organiation exists
func (t TimedDatabase) OrganizationExists(externalID string) (b bool, err error) {
	t.timeRequest("OrganizationExists", func() error {
		b, err = t.d.OrganizationExists(externalID)
		return err
	})
	return
}

// GetOrganizationName gets the name of an organization
func (t TimedDatabase) GetOrganizationName(externalID string) (name string, err error) {
	t.timeRequest("GetOrganizationName", func() error {
		name, err = t.d.GetOrganizationName(externalID)
		return err
	})
	return
}

// DeleteOrganization deletes an organization
func (t TimedDatabase) DeleteOrganization(externalID string) error {
	return t.timeRequest("DeleteOrganization", func() error {
		return t.d.DeleteOrganization(externalID)
	})
}

// AddFeatureFlag adds a feature flag to the organization
func (t TimedDatabase) AddFeatureFlag(externalID string, featureFlag string) error {
	return t.timeRequest("AddFeatureFlag", func() error {
		return t.d.AddFeatureFlag(externalID, featureFlag)
	})
}

// Close closes the database
func (t TimedDatabase) Close() error {
	return t.timeRequest("Close", func() error {
		return t.d.Close()
	})
}

// Truncate should only be used in testing
func (t TimedDatabase) Truncate() error {
	return t.d.(Truncater).Truncate()
}
