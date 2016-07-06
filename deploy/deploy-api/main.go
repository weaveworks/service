package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/mattes/migrate/migrate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/scope/common/instrument"
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
	s3RequestDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "scope",
		Name:      "s3_request_duration_nanoseconds",
		Help:      "Time spent doing S3 requests.",
	}, []string{"method", "status_code"})
)

func init() {
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(s3RequestDuration)
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
		s3URL         string
	)
	flag.StringVar(&listen, "listen", ":80", "HTTP server listen address")
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&databaseURL, "database", "", "URL of database")
	flag.StringVar(&migrationsDir, "migrations", "migrations", "Location of migrations directory")
	flag.StringVar(&s3URL, "s3", "", "URL for S3")
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

	parsedS3URL, err := url.Parse(s3URL)
	if err != nil {
		log.Fatalf("Valid URL for s3 required: %v", err)
		return
	}

	s3Config, err := common.AWSConfigFromURL(parsedS3URL)
	if err != nil {
		log.Fatalf("Failed to consturct AWS config: %v", err)
		return
	}

	d := &deployer{
		store:      common.NewDeployStore(db),
		s3:         s3.New(session.New(s3Config)),
		bucketName: strings.TrimPrefix(parsedS3URL.Path, "/"),
	}

	router := mux.NewRouter().StrictSlash(false).SkipClean(true)
	router.Path("/loadgen").Name("loadgen").Methods("GET").HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "OK")
	})
	router.Path("/metrics").Handler(prometheus.Handler())
	router.Methods("POST").Path("/api/deploy").HandlerFunc(withOrgID(d.deploy))
	router.Methods("GET").Path("/api/deploy").HandlerFunc(withOrgID(d.status))
	router.Methods("GET").Path("/api/deploy/{id}/log").HandlerFunc(withOrgID(d.getLog))
	router.Methods("POST").Path("/api/config/deploy").HandlerFunc(withOrgID(d.setConfig))
	router.Methods("GET").Path("/api/config/deploy").HandlerFunc(withOrgID(d.getConfig))
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
	store      *common.DeployStore
	s3         *s3.S3
	bucketName string
}

func (d *deployer) deploy(orgID string, w http.ResponseWriter, r *http.Request) {
	var request common.Deployment
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Errorf("Error decoding json body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := d.store.StoreNewDeployment(orgID, request); err != nil {
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
	deployments, err := d.store.GetDeployments(orgID)
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

	if err := d.store.StoreConfig(orgID, config); err != nil {
		log.Errorf("Error storing config: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *deployer) getConfig(orgID string, w http.ResponseWriter, r *http.Request) {
	config, err := d.store.GetConfig(orgID)
	if err == common.ErrNotFound {
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

func (d *deployer) getLog(orgID string, w http.ResponseWriter, r *http.Request) {
	deployID := mux.Vars(r)["id"]
	deployment, err := d.store.GetDeployment(orgID, deployID)
	if err == common.ErrNotFound {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Errorf("Error fetching deployment: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if deployment.LogKey == "" {
		http.Error(w, "No logs for deployment", http.StatusNotFound)
		return
	}

	var resp *s3.GetObjectOutput
	err = instrument.TimeRequest("Get", s3RequestDuration, func() error {
		var err error
		resp, err = d.s3.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(d.bucketName),
			Key:    aws.String(deployment.LogKey),
		})
		return err
	})
	if err != nil {
		log.Errorf("Error fetching log: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Errorf("Error copying log: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
