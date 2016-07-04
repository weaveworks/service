package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
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

func main() {
	var (
		listen      string
		logLevel    string
		databaseURL string
	)
	flag.StringVar(&listen, "listen", ":80", "HTTP server listen address")
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&databaseURL, "database", "", "URL of database")
	flag.Parse()

	if err := logging.Setup(logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
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

	w := &worker{
		db: db,
	}
	go w.loop()

	router := mux.NewRouter().StrictSlash(false).SkipClean(true)
	router.Path("/loadgen").Name("loadgen").Methods("GET").HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "OK")
	})
	router.Path("/metrics").Handler(prometheus.Handler())
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

type worker struct {
	db *sql.DB
}

const (
	maxBackoff     = 30 * time.Second
	initialBackoff = time.Second
)

func (w *worker) loop() {
	backoff := initialBackoff
	for {
		orgID, deployment, err := common.GetNextDeployment(w.db)
		if err != nil {
			log.Errorf("Failed to fetch next deploy: %v", err)
			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = initialBackoff

		log.Infof("Doing deployment %#v for organisation %s", deployment, orgID)
		if err := w.deploy(orgID, deployment); err != nil {
			log.Infof("Deployement failed: %v", err)
			common.UpdateDeploymentState(w.db, deployment.ID, common.Failed)

		} else {
			log.Infof("Deployement succeeded!")
			common.UpdateDeploymentState(w.db, deployment.ID, common.Success)
		}
	}
}

func (w *worker) deploy(orgid string, deployment *common.Deployment) error {
	config, err := common.GetConfig(w.db, orgid)
	if err != nil {
		return err
	}

	workingDir, err := ioutil.TempDir("/tmp", "deploy-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workingDir)

	if err := ioutil.WriteFile(workingDir+"/priv_key", []byte(config.RepoKey), os.FileMode(0700)); err != nil {
		return err
	}

	image := fmt.Sprintf("%s:%s", deployment.ImageName, deployment.Version)
	args := []string{config.RepoURL, config.RepoPath, "priv_key", config.KubeconfigPath, image}
	cmd := exec.Command("/bin/deploy.sh", args...)
	cmd.Dir = workingDir
	output, err := cmd.CombinedOutput()

	log.Infof("Output:\n%s", string(output))
	if err != nil {
		return fmt.Errorf("Command failed with exit code %s: %v", exitCode(err), err)
	}
	return nil
}

func exitCode(err error) string {
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return fmt.Sprintf("%d", status.ExitStatus())
		}
	}
	return "n/a"
}
