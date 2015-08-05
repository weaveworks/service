package main

import (
	"database/sql"
	"errors"

	"github.com/Sirupsen/logrus"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type organizationMapper interface {
	getOrganizationsHost(orgID string) (string, error)
}

type constantMapper struct {
	targetHost string
}

func (m *constantMapper) getOrganizationsHost(orgID string) (string, error) {
	return m.targetHost, nil
}

type dbMapper struct {
	db *sqlx.DB
}

type dbOrgHost struct {
	OrganizationID string `db:"organization_id"`
	Host           string `db:"host"`
}

func newDBMapper(dbURI string) (*dbMapper, error) {
	db, err := sqlx.Open("postgres", dbURI)
	if err != nil {
		return nil, err
	}
	return &dbMapper{db}, nil
}

func (m *dbMapper) getOrganizationsHost(orgID string) (string, error) {
	var host string
	transactionRunner := func(tx *sqlx.Tx) error {
		err := m.db.Get(&host, "SELECT org_host.host FROM org_host WHERE organization_id=$1;", orgID)
		if err == nil || err != sql.ErrNoRows {
			return err
		}

		// The organization wasn't assigned a host yet, let's find a free one and assign it
		err = tx.Get(
			&host,
			"SELECT hosts.host FROM hosts WHERE NOT EXISTS (SELECT org_host.host FROM org_host WHERE hosts.host = org_host.host) LIMIT 1;",
		)
		if err == sql.ErrNoRows {
			err = errors.New("dbMapper: ran out of hosts")
		}
		if err != nil {
			return err
		}

		toInsert := dbOrgHost{orgID, host}
		logrus.Infof("organization_mapper: adding mapping %v", toInsert)

		_, err = tx.NamedExec("INSERT INTO org_host VALUES (:organization_id, :host);", toInsert)

		return err
	}

	err := m.runTransaction(transactionRunner)

	return host, err
}

func (m *dbMapper) runTransaction(runner func(*sqlx.Tx) error) error {
	var (
		tx  *sqlx.Tx
		err error
	)

	if tx, err = m.db.Beginx(); err != nil {
		return err
	}

	if err = runner(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
