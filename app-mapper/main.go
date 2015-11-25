package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	scope "github.com/weaveworks/scope/xfer"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/weaveworks/service/common/instrument"
)

const (
	defaultAppImage = "weaveworks/scope:0.10.0"
	defaultDBURI    = "postgres://postgres@app-mapper-db/app_mapper?sslmode=disable"
)

var (
	defaultProvisionerClientTimeout = 1 * time.Second
)

type flags struct {
	listen                   string
	logLevel                 string
	mapperType               string
	dbURI                    string
	constantMapperTargetHost string
	appMapperDBHost          string
	authenticatorType        string
	authenticatorHost        string
	appProvisioner           string
	dockerAppImage           string
	dockerHost               string
	provisionerClientTimeout time.Duration
}

func parseFlags() *flags {
	f := flags{}
	flag.StringVar(&f.listen, "listen", ":80", "HTTP server listen address")
	flag.StringVar(&f.logLevel, "log-level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&f.mapperType, "mapper-type", "db", "Application mapper type to use: db | constant")
	flag.StringVar(&f.dbURI, "db-uri", defaultDBURI, "Where to find the db application mapper")
	flag.StringVar(&f.constantMapperTargetHost, "constant-mapper-target-host", "localhost:5450", "Host to be used by the constant mapper")
	flag.StringVar(&f.authenticatorType, "authenticator", "web", "What authenticator to use: web | mock")
	flag.StringVar(&f.authenticatorHost, "authenticator-host", "users:80", "Where to find web the authenticator service")
	flag.StringVar(&f.appProvisioner, "app-provisioner", "kubernetes", "What application provisioner to use: docker | kubernetes")
	flag.StringVar(&f.dockerAppImage, "docker-app-image", defaultAppImage, "Docker image to use by the application provisioner")
	flag.StringVar(&f.dockerHost, "docker-host", "", "Where to find the docker application provisioner")
	flag.DurationVar(&f.provisionerClientTimeout, "app-provisioner-timeout", defaultProvisionerClientTimeout, "Maximum time to wait for a response from the application provisioner")
	flag.Parse()
	return &f
}

func (f *flags) makeAuthenticator() authenticator {
	var a authenticator

	switch f.authenticatorType {
	case "mock":
		a = &mockAuthenticator{}
	case "web":
		a = &webAuthenticator{
			serverHost: f.authenticatorHost,
		}
	default:
		logrus.Fatal("Incorrect authenticator type: ", f.authenticatorType)
	}

	return a
}

func (f *flags) makeOrganizationMapper(db *sqlx.DB, p appProvisioner) organizationMapper {
	var m organizationMapper

	switch f.mapperType {
	case "constant":
		m = &constantMapper{
			targetHost: f.constantMapperTargetHost,
			isReady:    nil,
		}
	case "db":
		m = newDBMapper(db, p)
	default:
		logrus.Fatal("Incorrect mapper type: ", f.mapperType)
	}

	return m
}

func (f *flags) makeAppProvisioner() appProvisioner {

	var p appProvisioner

	args := []string{"--no-probe"}

	switch f.appProvisioner {
	case "docker":
		options := dockerProvisionerOptions{
			appConfig: docker.Config{
				Image: f.dockerAppImage,
				Cmd:   args,
			},
			hostConfig:    docker.HostConfig{},
			clientTimeout: f.provisionerClientTimeout,
		}
		var err error
		p, err = newDockerProvisioner(f.dockerHost, options)
		if err != nil {
			logrus.Fatal("Cannot initialize docker provisioner: ", err)
		}
	case "kubernetes":
		options := k8sProvisionerOptions{
			appContainer: kapi.Container{
				Name:  "scope",
				Image: f.dockerAppImage,
				Args:  args,
				Ports: []kapi.ContainerPort{
					{ContainerPort: scope.AppPort}},
			},
			clientTimeout: f.provisionerClientTimeout,
		}
		var err error
		p, err = newK8sProvisioner(options)
		if err != nil {
			logrus.Fatal("Cannot initialize kubernetes provisioner: ", err)
		}
	default:
		logrus.Fatal("Incorrect app provisioner: ", f.appProvisioner)
	}

	return p
}

func setupLogging(logLevel string) {
	logrus.SetOutput(os.Stderr)

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.SetLevel(level)
}

func main() {
	flags := parseFlags()
	setupLogging(flags.logLevel)

	authenticator := flags.makeAuthenticator()
	appProvisioner := flags.makeAppProvisioner()
	logrus.Info("Fetching application")
	err := appProvisioner.fetchApp()
	if err != nil {
		logrus.Error("Couldn't fetch application: ", err)
	}
	db, err := sqlx.Open("postgres", flags.dbURI)
	if err != nil {
		logrus.Fatal("Cannot initialize database client: ", err)
	}
	orgMapper := flags.makeOrganizationMapper(db, appProvisioner)
	probeStorage := newProbeDBStorage(db)

	router := mux.NewRouter()
	newProxy(authenticator, orgMapper, probeStorage).registerHandlers(router)
	newProbeObserver(authenticator, probeStorage).registerHandlers(router)
	http.Handle("/", instrument.Middleware(router, requestDuration)(router))
	http.Handle("/metrics", makePrometheusHandler())
	logrus.Infof("Listening on %s", flags.listen)
	logrus.Fatal(http.ListenAndServe(flags.listen, nil))
}
