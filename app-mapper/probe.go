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
	_, err := s.db.Exec("INSERT INTO probe VALUES ($1, $2, $3);", probeID, orgID, time.Now())
	return err
}

func newProbeObserver(a authenticator, g probeGetter) probeObserver {
	return probeObserver{a, g}
}

type probeObserver struct {
	authenticator authenticator
	getter        probeGetter
}

func (o probeObserver) registerHandlers(router *mux.Router) {
	router.Path("/api/org/{orgName}/probes").Methods("GET").Handler(authOrgHandler(o.authenticator,
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
