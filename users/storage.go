package main

import (
	"database/sql"
	"errors"
	"net/url"

	"github.com/Sirupsen/logrus"
)

var (
	ErrNotFound     = errors.New("Not found")
	ErrEmailIsTaken = errors.New("Email is already taken")
)

type Storage interface {
	CreateUser(email string) (*User, error)
	FindUserByID(id string) (*User, error)
	FindUserByEmail(email string) (*User, error)

	// Create a new user in an existing organization.
	// If the user already exists:
	// * in a *different* organization, this should return ErrEmailIsTaken.
	// * but is not approved, approve them into the organization.
	// * in the same organization, no-op.
	InviteUser(email, orgName string) (*User, error)

	ListUnapprovedUsers() ([]*User, error)
	ListOrganizationUsers(orgName string) ([]*User, error)

	// Approve the user for access. Should generate them a new organization.
	ApproveUser(id string) (*User, error)

	// Update the user's login token. Setting the token to "" should disable the
	// user's token.
	SetUserToken(id, token string) error

	FindOrganizationByProbeToken(probeToken string) (*Organization, error)

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
