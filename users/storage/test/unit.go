// +build !integration

package test

import (
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/storage"
)

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) storage.Database {
	storage.PasswordHashingCost = bcrypt.MinCost
	return storage.MustNew("memory://", "")
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T, db storage.Database) {
	// Noop for now
}
