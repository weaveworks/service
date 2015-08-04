package main

import (
	"database/sql"

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
	db          *sqlx.DB
	provisioner appProvisioner
}

type dbOrgHost struct {
	OrganizationID string `db:"organization_id"`
	Hostname       string `db:"hostname"`
}

func newDBMapper(dbURI string, p appProvisioner) (*dbMapper, error) {
	db, err := sqlx.Open("postgres", dbURI)
	if err != nil {
		return nil, err
	}
	return &dbMapper{db, p}, nil
}

func (m *dbMapper) getOrganizationsHost(orgID string) (string, error) {
	var host string
	transactionRunner := func(tx *sqlx.Tx) error {
		err := m.db.Get(&host, "SELECT org_hostname.hostname FROM org_hostname WHERE organization_id=$1;", orgID)
		if err == nil || err != sql.ErrNoRows {
			return err
		}

		// The organization wasn't assigned a host yet, let's allocate one and assign it
		logrus.Infof("organization mapper: provisioning app for organization %v", orgID)
		host, err = m.provisioner.runApp(orgID)
		if err != nil {
			return err
		}

		toInsert := dbOrgHost{orgID, host}
		logrus.Infof("organization mapper: adding mapping %v", toInsert)

		_, err = tx.NamedExec("INSERT INTO org_hostname VALUES (:organization_id, :hostname);", toInsert)

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
