package main

import (
	"time"

	"github.com/jmoiron/sqlx"
)

const probeIDHeaderName = "X-Scope-Probe-ID"

type probe struct {
	ID       string    `db:"probe_id"`
	OrgID    string    `db:"organization_id"`
	LastSeen time.Time `db:"last_seen"`
}

type probeStorage interface {
	getProbesFromOrg(orgID string) ([]probe, error)
	bumpProbeLastSeen(probeID string, orgID string) error
}

type probeDBStorage struct {
	db *sqlx.DB
}

func newProbeDBStorage(db *sqlx.DB) probeDBStorage {
	return probeDBStorage{db}
}

func (s probeDBStorage) getProbesFromOrg(orgID string) ([]probe, error) {
	var probes []probe
	err := s.db.Select(&probes, "SELECT * FROM probe WHERE organization_id=$1;", orgID)
	return probes, err
}

func (s probeDBStorage) bumpProbeLastSeen(probeID string, orgID string) error {
	_, err := s.db.Exec("INSERT INTO probe VALUES ($1, $2, $3);", probeID, orgID, time.Now())
	return err
}
