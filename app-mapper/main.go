package main

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/weaveworks/service/common/instrument"
)

func main() {
	flags := parseFlags()
	setupLogging(flags.logLevel)

	authenticator := flags.getAuthenticator()
	appProvisioner := flags.getAppProvisioner()
	logrus.Info("Fetching application")
	err := appProvisioner.fetchApp()
	if err != nil {
		logrus.Error("Couldn't fetch application: ", err)
	}
	db, err := sqlx.Open("postgres", flags.dbURI)
	if err != nil {
		logrus.Fatal("Cannot initialize database client: ", err)
	}
	orgMapper := flags.getOrganizationMapper(db, appProvisioner)
	probeStorage := newProbeDBStorage(db)

	router := mux.NewRouter()
	newProxy(authenticator, orgMapper, probeStorage).registerHandlers(router)
	newProbeObserver(authenticator, probeStorage).registerHandlers(router)
	http.Handle("/", instrument.Middleware(router, requestDuration)(router))
	http.Handle("/metrics", makePrometheusHandler())
	logrus.Infof("Listening on %s", flags.listen)
	logrus.Fatal(http.ListenAndServe(flags.listen, nil))
}
