// +build integration

package sql

import (
	"net/url"
	"testing"

	"github.com/weaveworks/service/flux-api/db"
)

var (
	dbURL = "postgres://postgres@postgres:5432?sslmode=disable"
)

func newDB(t *testing.T) *DB {
	u, err := url.Parse(dbURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Migrate(dbURL, "../../db/migrations/postgres"); err != nil {
		t.Fatal(err)
	}
	db, err := New(u.Scheme, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
