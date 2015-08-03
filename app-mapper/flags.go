package main

import (
	"flag"

	"github.com/Sirupsen/logrus"
)

type flags struct {
	logLevel                 string
	mapperType               string
	dbMapperURI              string
	constantMapperTargetHost string
	appMapperDBHost          string
	authenticatorType        string
	authenticatorHost        string
}

func parseFlags() *flags {
	f := flags{}
	flag.StringVar(&f.logLevel, "log-level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&f.mapperType, "mapper-type", "db", "Application mapper type to use: db | constant")
	flag.StringVar(&f.dbMapperURI, "db-mapper-uri", "postgres://postgres@app-mapper-db.weave.local/app_mapper?sslmode=disable", "Where to contact the database")
	flag.StringVar(&f.constantMapperTargetHost, "constant-mapper-target-host", "localhost:5450", "Host to be used by the constant mapper")
	flag.StringVar(&f.authenticatorType, "authenticator-type", "web", "Authenticator type to use: web | mock")
	flag.StringVar(&f.authenticatorHost, "authenticator-host", "users.weave.local:80", "Where to contact the authenticator service")
	flag.Parse()

	if f.mapperType != "db" && f.mapperType != "constant" {
		logrus.Fatal("Incorrect mapper type:", f.mapperType)
	}

	if f.authenticatorType != "web" && f.authenticatorType != "mock" {
		logrus.Fatal("Incorrect authenticator type:", f.authenticatorType)
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

func (f *flags) getOrganizationMapper() organizationMapper {
	if f.mapperType == "constant" {
		return &constantMapper{
			targetHost: f.constantMapperTargetHost,
		}
	}

	m, err := newDBMapper(f.dbMapperURI)
	if err != nil {
		logrus.Fatal("Cannot initialize database client:", err)
	}
	return m
}
