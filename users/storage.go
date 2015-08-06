package main

import (
	"database/sql"
	"errors"
	"net/url"

	"github.com/Sirupsen/logrus"
)

var (
	ErrNotFound = errors.New("Not found")
)

type Storage interface {
	CreateUser(email string) (*User, error)
	FindUserByID(id string) (*User, error)
	FindUserByEmail(email string) (*User, error)

	ListUnapprovedUsers() ([]*User, error)
	// Approve the user for access. Should generate them a new organization.
	ApproveUser(id string) (*User, error)

	// Update the user's login token. Setting the token to "" should disable the
	// user's token.
	SetUserToken(id, token string) error

	FindOrganizationByName(name string) (*Organization, error)

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
