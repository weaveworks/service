// +build !integration

package dbtest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/common/logging"
	"github.com/weaveworks/service/users/db"
	_ "github.com/weaveworks/service/users/db/memory" // Load the memory db driver
)

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) db.DB {
	require.NoError(t, logging.Setup("debug"))
	db.PasswordHashingCost = bcrypt.MinCost
	return db.MustNew("memory://", "")
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T, db db.DB) {
	// Noop for now
}
