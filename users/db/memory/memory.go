package memory

import (
	"sync"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

// DB is an in-memory database for testing, and local development
type DB struct {
	users               map[string]*users.User
	organizations       map[string]*users.Organization
	memberships         map[string][]string
	logins              map[string]*login.Login
	apiTokens           map[string]*users.APIToken
	passwordHashingCost int
	mtx                 sync.Mutex
}

// New creates a new in-memory database
func New(_, _ string, passwordHashingCost int) (*DB, error) {
	return &DB{
		users:               make(map[string]*users.User),
		organizations:       make(map[string]*users.Organization),
		memberships:         make(map[string][]string),
		logins:              make(map[string]*login.Login),
		apiTokens:           make(map[string]*users.APIToken),
		passwordHashingCost: passwordHashingCost,
	}, nil
}

// Truncate clears all the data. Should only be used in tests!
func (d *DB) Truncate() error {
	*d = DB{
		users:               make(map[string]*users.User),
		organizations:       make(map[string]*users.Organization),
		memberships:         make(map[string][]string),
		logins:              make(map[string]*login.Login),
		apiTokens:           make(map[string]*users.APIToken),
		passwordHashingCost: d.passwordHashingCost,
	}
	return nil
}

// Close finishes using the db. Noop.
func (d *DB) Close() error {
	return nil
}
