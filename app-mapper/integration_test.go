// +build integration

package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var testDB *sqlx.DB

func init() {
	var err error
	testDB, err = sqlx.Open("postgres", defaultDBURI)
	if err != nil {
		logrus.Fatal("Cannot initialize database client: ", err)
	}
	testProbeStorage = newProbeDBStorage(testDB)
}
