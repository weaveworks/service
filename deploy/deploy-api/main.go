package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/mattes/migrate/migrate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/middleware"

	"github.com/weaveworks/service/common/logging"
	"github.com/weaveworks/service/deploy/common"
)

var (
	requestDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Name:      "request_duration_seconds",
		Help:      "Time (in seconds) spent serving HTTP requests.",
	}, []string{"method", "route", "status_code", "ws"})
	orgPrefix = regexp.MustCompile("^/api/app/[^/]+")
)

func init() {
	prometheus.MustRegister(requestDuration)
}

const (
	orgIDHeader = "X-Scope-OrgID"
)

func main() {
	var (
		listen        string
		logLevel      string
		databaseURL   string
		migrationsDir string
	)
	flag.StringVar(&listen, "listen", ":80", "HTTP server listen address")
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&databaseURL, "database", "", "URL of database")
	flag.StringVar(&migrationsDir, "migrations", "migrations", "Location of migrations directory")
	flag.Parse()

	if err := logging.Setup(logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	log.Infof("Running Database Migrations...")
	if errs, ok := migrate.UpSync(databaseURL, migrationsDir); !ok {
		for _, err := range errs {
			log.Error(err)
		}
		log.Fatal("Database migrations failed")
	}

	dbURL, err := url.Parse(databaseURL)
	if err != nil {
		log.Fatalf("Error parsing db url: %v", err)
		return
	}

	db, err := sql.Open(dbURL.Scheme, databaseURL)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
		return
	}

	d := &deployer{
		DB: db,
	}

	router := mux.NewRouter().StrictSlash(false).SkipClean(true)
	router.Path("/loadgen").Name("loadgen").Methods("GET").HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "OK")
	})
	router.Path("/metrics").Handler(prometheus.Handler())
	router.Methods("POST").Path("/api/deploy/new").HandlerFunc(withOrgID(d.deploy))
	router.Methods("GET").Path("/api/deploy/status").HandlerFunc(withOrgID(d.status))
	router.Methods("POST").Path("/api/deploy/config").HandlerFunc(withOrgID(d.setConfig))
	router.Methods("GET").Path("/api/deploy/config").HandlerFunc(withOrgID(d.getConfig))
	log.Infof("Listening on %s", listen)
	log.Fatal(http.ListenAndServe(
		listen,
		middleware.Merge(
			middleware.Instrument{
				RouteMatcher: router,
				Duration:     requestDuration,
			},
			middleware.Logging,
		).Wrap(router),
	))
}

func withOrgID(f func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get(orgIDHeader)
		if orgID == "" {
			http.Error(w, "no org ID header", http.StatusUnauthorized)
			return
		}
		f(orgID, w, r)
	}
}

type deployer struct {
	DB *sql.DB
}

func (d *deployer) deploy(orgID string, w http.ResponseWriter, r *http.Request) {
	var request common.Deployment
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Errorf("Error decoding json body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := common.StoreNewDeployment(d.DB, orgID, request); err != nil {
		log.Errorf("Error doing insert: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type statusResponse struct {
	Deployments []common.Deployment `json:"deployments"`
}

func (d *deployer) status(orgID string, w http.ResponseWriter, r *http.Request) {
	deployments, err := common.GetDeployments(d.DB, orgID)
	if err != nil {
		log.Errorf("Error getting deployments: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := statusResponse{
		Deployments: deployments,
	}
	if err := json.NewEncoder(w).Encode(&response); err != nil {
		log.Errorf("Error encoding json: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (d *deployer) setConfig(orgID string, w http.ResponseWriter, r *http.Request) {
	var config common.Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		log.Errorf("Error decoding json body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := common.StoreConfig(d.DB, orgID, config); err != nil {
		log.Errorf("Error storing config: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *deployer) getConfig(orgID string, w http.ResponseWriter, r *http.Request) {
	config, err := common.GetConfig(d.DB, orgID)
	if err == common.ErrNoConfig {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		log.Errorf("Error fetching config: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(config); err != nil {
		log.Errorf("Error encoding config: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
