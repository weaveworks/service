// +build !integration

package dbtest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/notebooks/db"
)

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) db.DB {
	database, err := db.New(dbconfig.New("memory://", "", ""))
	require.NoError(t, err)
	return database
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T, database db.DB) {
	require.NoError(t, database.Close())
}
