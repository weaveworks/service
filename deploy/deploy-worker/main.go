package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
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

func main() {
	var (
		listen      string
		logLevel    string
		databaseURL string
		s3URL       string
	)
	flag.StringVar(&listen, "listen", ":80", "HTTP server listen address")
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&databaseURL, "database", "", "URL of database")
	flag.StringVar(&s3URL, "s3", "", "URL of S3")
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

	w := &worker{
		store:      common.NewDeployStore(db),
		s3:         s3.New(session.New(s3Config)),
		bucketName: strings.TrimPrefix(parsedS3URL.Path, "/"),
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
	store      *common.DeployStore
	s3         *s3.S3
	bucketName string
}

const (
	maxBackoff     = 30 * time.Second
	initialBackoff = time.Second
)

func (w *worker) loop() {
	backoff := initialBackoff
	for {
		orgID, deployment, err := w.store.GetNextDeployment()
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

		// Run the deployment, storing the logs in memory
		log.Infof("Doing deployment %#v for organisation %s", deployment, orgID)
		output, err := w.deploy(orgID, deployment)

		// Write logs to S3
		logKey := ""
		if output != nil {
			var s3err error
			logKey, s3err = w.writeToS3(orgID, deployment.ID, output)
			if s3err != nil {
				log.Errorf("Send log to S3 failed: %v", s3err)
			}
		}

		// Update deplop
		if err != nil {
			log.Infof("Deployement failed: %v", err)
			w.store.UpdateDeploymentState(deployment.ID, common.Failed, logKey)
		} else {
			log.Infof("Deployement succeeded!")
			w.store.UpdateDeploymentState(deployment.ID, common.Success, logKey)
		}
	}
}

func (w *worker) deploy(orgid string, deployment *common.Deployment) ([]byte, error) {
	config, err := w.store.GetConfig(orgid)
	if err != nil {
		return nil, err
	}

	workingDir, err := ioutil.TempDir("/tmp", "deploy-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workingDir)

	if err := ioutil.WriteFile(workingDir+"/priv_key", []byte(config.RepoKey), os.FileMode(0700)); err != nil {
		return nil, err
	}

	image := fmt.Sprintf("%s:%s", deployment.ImageName, deployment.Version)
	args := []string{config.RepoURL, config.RepoPath, "priv_key", config.KubeconfigPath, image}
	cmd := exec.Command("/bin/deploy.sh", args...)
	cmd.Dir = workingDir
	output := bytes.Buffer{}
	cmd.Stdout = io.MultiWriter(&output, os.Stdout)
	cmd.Stderr = io.MultiWriter(&output, os.Stderr)
	if err := cmd.Run(); err != nil {
		return output.Bytes(), fmt.Errorf("Command failed with exit code %s: %v", exitCode(err), err)
	}
	return output.Bytes(), nil
}

func exitCode(err error) string {
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return fmt.Sprintf("%d", status.ExitStatus())
		}
	}
	return "n/a"
}

func (w *worker) writeToS3(orgID, deployID string, output []byte) (string, error) {
	key := fmt.Sprintf("%s/deploy-%s.log", orgID, deployID)
	err := instrument.TimeRequest("Put", s3RequestDuration, func() error {
		var err error
		_, err = w.s3.PutObject(&s3.PutObjectInput{
			Body:   bytes.NewReader(output),
			Bucket: aws.String(w.bucketName),
			Key:    aws.String(key),
		})
		return err
	})
	if err != nil {
		return "", err
	}
	return key, nil
}
