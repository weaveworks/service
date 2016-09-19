// +build !integration

package dbtest

import (
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/db"
	_ "github.com/weaveworks/service/users/db/memory" // Load the memory db driver
)

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) db.DB {
	db.PasswordHashingCost = bcrypt.MinCost
	return db.MustNew("memory://", "")
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T, db db.DB) {
	// Noop for now
}
