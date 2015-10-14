package main

import (
	"flag"
	"time"

	"github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/jmoiron/sqlx"
)

const (
	defaultAppImage = "weaveworks/scope:0.8.0"
	defaultDBURI    = "postgres://postgres@app-mapper-db.weave.local/app_mapper?sslmode=disable"
)

var (
	defaultDockerClientTimeout = 1 * time.Second
	defaultDockerRunTimeout    = 2 * time.Second
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
	dockerAppImage           string
	dockerHost               string
	dockerClientTimeout      time.Duration
	dockerRunTimeout         time.Duration
}

func parseFlags() *flags {
	f := flags{}
	flag.StringVar(&f.listen, "listen", ":80", "HTTP server listen address")
	flag.StringVar(&f.logLevel, "log-level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&f.mapperType, "mapper-type", "db", "Application mapper type to use: db | constant")
	flag.StringVar(&f.dbURI, "db-uri", defaultDBURI, "Where to contact the database")
	flag.StringVar(&f.constantMapperTargetHost, "constant-mapper-target-host", "localhost:5450", "Host to be used by the constant mapper")
	flag.StringVar(&f.authenticatorType, "authenticator-type", "web", "Authenticator type to use: web | mock")
	flag.StringVar(&f.authenticatorHost, "authenticator-host", "users.weave.local:80", "Where to find the authenticator service")
	flag.StringVar(&f.dockerAppImage, "docker-app-image", defaultAppImage, "Docker image to use by the docker app provisioner")
	flag.StringVar(&f.dockerHost, "docker-host", "tcp://swarm-master.weave.local:4567", "Where to find the docker service")
	flag.DurationVar(&f.dockerClientTimeout, "docker-client-timeout", defaultDockerClientTimeout, "Maximum time to wait for a response from docker")
	flag.DurationVar(&f.dockerRunTimeout, "docker-run-timeout", defaultDockerRunTimeout, "Maximum time to wait for an app to run")
	flag.Parse()

	if f.mapperType != "db" && f.mapperType != "constant" {
		logrus.Fatal("Incorrect mapper type: ", f.mapperType)
	}

	if f.authenticatorType != "web" && f.authenticatorType != "mock" {
		logrus.Fatal("Incorrect authenticator type: ", f.authenticatorType)
	}

	return &f
}

func (f *flags) getAuthenticator() authenticator {
	if f.authenticatorType == "mock" {
		return &mockAuthenticator{}
	}

	return &webAuthenticator{
		serverHost: f.authenticatorHost,
	}
}

func (f *flags) getOrganizationMapper(db *sqlx.DB, p appProvisioner) organizationMapper {
	if f.mapperType == "constant" {
		return &constantMapper{
			targetHost: f.constantMapperTargetHost,
			isReady:    nil,
		}
	}
	m := newDBMapper(db, p)
	return m
}

func (f *flags) getAppProvisioner() appProvisioner {
	options := dockerProvisionerOptions{
		appConfig: docker.Config{
			Image: defaultAppImage,
			Cmd:   []string{"--no-probe"},
		},
		hostConfig:    docker.HostConfig{},
		runTimeout:    f.dockerRunTimeout,
		clientTimeout: f.dockerClientTimeout,
	}
	m, err := newDockerProvisioner(f.dockerHost, options)
	if err != nil {
		logrus.Fatal("Cannot initialize docker provisioner: ", err)
	}
	return m
}
