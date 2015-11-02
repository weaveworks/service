package main

import (
	"flag"
	"time"

	"github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/jmoiron/sqlx"
	scope "github.com/weaveworks/scope/xfer"
	k8sAPI "k8s.io/kubernetes/pkg/api"
)

const (
	defaultAppImage = "weaveworks/scope:0.8.0"
	defaultDBURI    = "postgres://postgres@app-mapper-db/app_mapper?sslmode=disable"
)

var (
	defaultProvisionerClientTimeout = 1 * time.Second
	defaultProvisionerRunTimeout    = 2 * time.Second
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
	provisionerRunTimeout    time.Duration
}

func parseFlags() *flags {
	f := flags{}
	flag.StringVar(&f.listen, "listen", ":80", "HTTP server listen address")
	flag.StringVar(&f.logLevel, "log-level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&f.mapperType, "mapper-type", "db", "Application mapper type to use: db | constant")
	flag.StringVar(&f.dbURI, "db-uri", defaultDBURI, "Where to find the db application mapper")
	flag.StringVar(&f.constantMapperTargetHost, "constant-mapper-target-host", "localhost:5450", "Host to be used by the constant mapper")
	flag.StringVar(&f.authenticatorType, "authenticator", "web", "What authenticator to use: web | mock")
	flag.StringVar(&f.authenticatorHost, "authenticator-host", "users.weave.local:80", "Where to find web the authenticator service")
	flag.StringVar(&f.appProvisioner, "app-provisioner", "kubernetes", "What application provisioner to use: docker | kubernetes")
	flag.StringVar(&f.dockerAppImage, "docker-app-image", defaultAppImage, "Docker image to use by the application provisioner")
	flag.StringVar(&f.dockerHost, "docker-host", "", "Where to find the docker application provisioner")
	flag.DurationVar(&f.provisionerClientTimeout, "app-provisioner-timeout", defaultProvisionerClientTimeout, "Maximum time to wait for a response from the application provisioner")
	flag.DurationVar(&f.provisionerRunTimeout, "app-provisioner-run-timeout", defaultProvisionerRunTimeout, "Maximum time the application provisioner will wait for an application to start running")
	flag.Parse()

	if f.mapperType != "db" && f.mapperType != "constant" {
		logrus.Fatal("Incorrect mapper type: ", f.mapperType)
	}

	if f.authenticatorType != "web" && f.authenticatorType != "mock" {
		logrus.Fatal("Incorrect authenticator type: ", f.authenticatorType)
	}

	if f.appProvisioner != "docker" && f.appProvisioner != "kubernetes" {
		logrus.Fatal("Incorrect app provisioner: ", f.appProvisioner)
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
	generalOptions := appProvisionerOptions{
		runTimeout:    f.provisionerRunTimeout,
		clientTimeout: f.provisionerClientTimeout,
	}
	args := []string{"--no-probe"}

	if f.appProvisioner == "docker" {
		options := dockerProvisionerOptions{
			appConfig: docker.Config{
				Image: f.dockerAppImage,
				Cmd:   args,
			},
			hostConfig:            docker.HostConfig{},
			appProvisionerOptions: generalOptions,
		}
		p, err := newDockerProvisioner(f.dockerHost, options)
		if err != nil {
			logrus.Fatal("Cannot initialize docker provisioner: ", err)
		}
		return p
	}

	options := k8sProvisionerOptions{
		appContainer: k8sAPI.Container{
			Name:  "scope",
			Image: f.dockerAppImage,
			Args:  args,
			Ports: []k8sAPI.ContainerPort{
				k8sAPI.ContainerPort{ContainerPort: scope.AppPort}},
		},
		appProvisionerOptions: generalOptions,
	}

	p, err := newK8sProvisioner(options)
	if err != nil {
		logrus.Fatal("Cannot initialize kubernetes provisioner: ", err)
	}
	return p
}
