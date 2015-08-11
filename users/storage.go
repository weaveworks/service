package main

import (
	"database/sql"
	"errors"
	"net/url"

	"github.com/Sirupsen/logrus"
)

var (
	errNotFound     = errors.New("Not found")
	errEmailIsTaken = validationErrorf("Email is already taken")
)

type database interface {
	CreateUser(email string) (*user, error)
	FindUserByID(id string) (*user, error)
	FindUserByEmail(email string) (*user, error)

	// Create a new user in an existing organization.
	// If the user already exists:
	// * in a *different* organization, this should return errEmailIsTaken.
	// * but is not approved, approve them into the organization.
	// * in the same organization, no-op.
	InviteUser(email, orgName string) (*user, error)

	// Ensure a user is deleted. If they do not exist, return success.
	DeleteUser(email string) error

	ListUnapprovedUsers() ([]*user, error)
	ListOrganizationUsers(orgName string) ([]*user, error)

	// Approve the user for access. Should generate them a new organization.
	ApproveUser(id string) (*user, error)

	// Update the user's login token. Setting the token to "" should disable the
	// user's token.
	SetUserToken(id, token string) error

	FindOrganizationByProbeToken(probeToken string) (*organization, error)
	RenameOrganization(oldName, newName string) error

	Close() error
}

func setupStorage(databaseURI string) {
	u, err := url.Parse(databaseURI)
	if err != nil {
		logrus.Fatal(err)
	}
	switch u.Scheme {
	case "postgres":
		db, err := sql.Open(u.Scheme, databaseURI)
		if err != nil {
			logrus.Fatal(err)
		}
		storage = &pgStorage{db}
	case "memory":
		storage = newMemoryStorage()
	default:
		logrus.Fatalf("Unknown database type: %s", u.Scheme)
	}
}
