package main

import (
	"database/sql"

	"github.com/Sirupsen/logrus"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type hostInfo struct {
	HostName string `db:"hostname"`
	IsReady  bool   `db:"is_ready"`
}

type organizationMapper interface {
	getOrganizationsHost(orgID string) (hostInfo, error)
}

type constantMapper struct {
	targetHost string
	isReady    *bool
}

func (m *constantMapper) getOrganizationsHost(orgID string) (hostInfo, error) {
	isReady := m.isReady == nil || *m.isReady
	return hostInfo{m.targetHost, isReady}, nil
}

type dbMapper struct {
	db          *sqlx.DB
	provisioner appProvisioner
}

func newDBMapper(db *sqlx.DB, p appProvisioner) *dbMapper {
	return &dbMapper{db, p}
}

func (m *dbMapper) getOrganizationsHost(orgID string) (hostInfo, error) {
	var hostInfo hostInfo
	transactionRunner := func(tx *sqlx.Tx) error {
		err := tx.Get(&hostInfo, "SELECT hostname, is_ready FROM org_hostname WHERE organization_id=$1;", orgID)
		if (err == nil && hostInfo.IsReady) || (err != nil && err != sql.ErrNoRows) {
			return err
		}

		switch {

		case err == sql.ErrNoRows:
			// The organization wasn't assigned a host yet, let's allocate one and assign it
			logrus.Infof("organization mapper: provisioning app for organization %q", orgID)
			hostInfo.HostName, err = m.provisioner.runApp(orgID)
			if err != nil {
				return err
			}

			// Most times the app is ready right away, let's confirm
			ready, err2 := m.provisioner.isAppReady(orgID)
			hostInfo.IsReady = err2 == nil && ready

			logrus.Infof("organization mapper: adding mapping %q -> %q", orgID, hostInfo.HostName)
			_, err = tx.Exec(
				"INSERT INTO org_hostname VALUES ($1, $2, $3);",
				orgID, hostInfo.HostName, hostInfo.IsReady)

		case !hostInfo.IsReady:
			// The organization was assigned a host but, last time we checked, it wasn't ready.
			// Let's check again
			ready, err2 := m.provisioner.isAppReady(orgID)
			hostInfo.IsReady = err2 == nil && ready
			if !hostInfo.IsReady {
				// Still not ready, no need to record it in the DB
				return nil
			}
			logrus.Infof("organization mapper: marking mapping orgID %q -> %q as ready", orgID, hostInfo.HostName)
			_, err = tx.Exec(
				"UPDATE org_hostname SET is_ready=true WHERE organization_id=$1 and hostname=$2;",
				orgID, hostInfo.HostName)
		}

		return err
	}

	err := m.runTransaction(transactionRunner)
	return hostInfo, err
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
		logrus.Warnf("organization mapper: failure during transaction: %v", err)
		if err2 := tx.Rollback(); err2 != nil {
			logrus.Warnf("organization mapper: transaction rollback: %v", err2)
		}
		return err
	}

	return tx.Commit()
}
