package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
)

type probe struct {
	ID       string    `db:"probe_id" json:"id"`
	LastSeen time.Time `db:"last_seen" json:"lastSeen"`
}

type probeGetter interface {
	getProbesFromOrg(orgID string) ([]probe, error)
}

type probeBumper interface {
	bumpProbeLastSeen(probeID string, orgID string) error
}

type probeDBStorage struct {
	db *sqlx.DB
}

func newProbeDBStorage(db *sqlx.DB) *probeDBStorage {
	return &probeDBStorage{db}
}

func (s *probeDBStorage) getProbesFromOrg(orgID string) ([]probe, error) {
	var probes []probe
	err := s.db.Select(&probes, "SELECT probe_id, last_seen FROM probe WHERE organization_id=$1;", orgID)
	return probes, err
}

func (s *probeDBStorage) bumpProbeLastSeen(probeID string, orgID string) error {
	transactionRunner := func(tx *sqlx.Tx) error {
		var wasSeenBefore bool
		err := tx.Get(&wasSeenBefore, "SELECT EXISTS(SELECT 1 FROM probe WHERE probe_id=$1 AND organization_id=$2);", probeID, orgID)
		if err != nil {
			return err
		}
		if wasSeenBefore {
			_, err = tx.Exec("UPDATE probe SET last_seen=$1 WHERE probe_id=$2 AND organization_id=$3;", time.Now(), probeID, orgID)
		} else {
			_, err = tx.Exec("INSERT INTO probe VALUES ($1, $2, $3);", probeID, orgID, time.Now())
		}
		return err
	}
	return runTransaction(s.db, transactionRunner)
}

func newProbeObserver(a authenticator, g probeGetter) probeObserver {
	return probeObserver{a, g}
}

type probeObserver struct {
	authenticator authenticator
	getter        probeGetter
}

func (o probeObserver) registerHandlers(router *mux.Router) {
	router.Path("/api/org/{orgName}/probes").Name("api_org_probes").Methods("GET").Handler(authOrgHandler(o.authenticator,
		func(r *http.Request) string { return mux.Vars(r)["orgName"] },
		func(w http.ResponseWriter, r *http.Request, orgID string) {
			probes, err := o.getter.getProbesFromOrg(orgID)
			if err != nil {
				logrus.Errorf("probe: cannot access probes from org with id %q: %v", orgID, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if err = json.NewEncoder(w).Encode(probes); err != nil {
				logrus.Errorf("probe: cannot encode probes to json: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			w.WriteHeader(http.StatusOK)
		},
	))
}
