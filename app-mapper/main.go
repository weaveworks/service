package main

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
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
	http.Handle("/", router)
	logrus.Info("Listening on :80")
	handler := loggingHandler(http.DefaultServeMux)
	logrus.Fatal(http.ListenAndServe(":80", handler))
}

func loggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logrus.Infof("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}
