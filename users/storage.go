package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/url"

	"github.com/Sirupsen/logrus"
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/mattes/migrate/migrate"
)

var (
	errNotFound       = errors.New("Not found")
	errEmailIsTaken   = validationErrorf("Email is already taken")
	errOrgNameIsTaken = validationErrorf("Organization name is already taken")
)

type database interface {
	// Create a user. The driver should set ID to some default only when it is "".
	CreateUser(id, email string) (*user, error)

	findUserByIDer
	FindUserByEmail(email string) (*user, error)
	FindUserByLogin(provider, id string) (*user, error)

	// AddLoginToUser adds an entry denoting this user is linked to a
	// remote login. e.g. if a user logs in via github this maps our
	// account to the github account.
	// Note: Must be idempotent!
	AddLoginToUser(userID, provider, id string, session json.RawMessage) error

	// DetachLoginFromUser removes all entries an entry denoting this
	// user is linked to the remote login.
	DetachLoginFromUser(userID, provider string) error

	// Create a new user in an existing organization.
	// If the user already exists:
	// * in a *different* organization, this should return errEmailIsTaken.
	// * but is not approved, approve them into the organization.
	// * in the same organization, no-op.
	InviteUser(email, orgName string) (*user, error)

	// Ensure a user is deleted. If they do not exist, return success.
	DeleteUser(email string) error

	ListUsers(...filter) ([]*user, error)
	ListOrganizationUsers(orgName string) ([]*user, error)

	// Approve the user for access. Should generate them a new organization.
	ApproveUser(id string) (*user, error)

	// Set the admin flag of a user
	SetUserAdmin(id string, value bool) error

	// Update the user's login token. Setting the token to "" should disable the
	// user's token.
	SetUserToken(id, token string) error

	// Update the user's first login timestamp. Should be called the first time a user logs in (i.e. if FirstLoginAt.IsZero())
	SetUserFirstLoginAt(id string) error

	// Create a new organization owned by the user. The name will be unique and randomly generated.
	CreateOrganization(ownerID string) (*organization, error)
	FindOrganizationByProbeToken(probeToken string) (*organization, error)
	RenameOrganization(oldName, newName string) error

	Close() error
}

type findUserByIDer interface {
	FindUserByID(id string) (*user, error)
}

func mustNewDatabase(databaseURI, migrationsDir string) database {
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
	var storage database
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
	return storage
}

// storageAuth lets you authenticate a user against the database
type storageAuth struct {
	database database
}

func (a *storageAuth) Flags(flags *flag.FlagSet) {}

func (a *storageAuth) Link(id string, r *http.Request) map[string]string {
	return nil
}

// Login checks this user exists on the oauth provider
func (a *storageAuth) Login(r *http.Request) (string, string, json.RawMessage, error) {
	email := r.FormValue("email")
	token := r.FormValue("token")
	switch {
	case email == "":
		return "", "", nil, validationErrorf("Email cannot be blank")
	case token == "":
		return "", "", nil, validationErrorf("Token cannot be blank")
	}

	u, err := a.database.FindUserByEmail(email)
	if err != nil {
		if err == errNotFound {
			err = errInvalidAuthenticationData
		}
		return "", "", nil, err
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	if !u.CompareToken(token) {
		return "", "", nil, errInvalidAuthenticationData
	}

	if err := a.database.SetUserToken(u.ID, ""); err != nil {
		return "", "", nil, err
	}

	session, err := json.Marshal(u.ID)
	if err != nil {
		return "", "", nil, err
	}

	return u.ID, u.Email, session, nil
}

// Username fetches a user's username on the remote service, for displaying *which* account this is linked with.
func (a *storageAuth) Username(session json.RawMessage) (string, error) {
	var id string
	if err := json.Unmarshal(session, &id); err != nil {
		return "", err
	}

	u, err := a.database.FindUserByID(id)
	if err != nil {
		return "", err
	}

	return u.Email, nil
}
