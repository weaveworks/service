package main

import (
	"database/sql"
	"errors"
	"net/url"

	"github.com/Sirupsen/logrus"
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/mattes/migrate/migrate"
)

var (
	errNotFound     = errors.New("Not found")
	errEmailIsTaken = validationErrorf("Email is already taken")
)

type database interface {
	CreateUser(email string) (*user, error)
	findUserByIDer
	FindUserByEmail(email string) (*user, error)

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
