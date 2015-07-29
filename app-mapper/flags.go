package main

import (
	"flag"
)

type flags struct {
	logLevel                 string
	mapperType               string
	constantMapperTargetHost string
	appMapperDBHost          string
	authenticatorType        string
	authenticatorHost        string
}

func parseFlags() *flags {
	f := flags{}
	flag.StringVar(&f.logLevel, "log-level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&f.mapperType, "mapper-type", "db", "Application mapper type to use: db | constant")
	flag.StringVar(&f.constantMapperTargetHost, "constant-mapper-target-host", "localhost:5450", "Host to be used by the constant mapper")
	flag.StringVar(&f.authenticatorType, "authenticator-type", "web", "Authenticator type to use: web | mock")
	flag.StringVar(&f.authenticatorHost, "authenticator-host", "users.weave.local:80", "Where to contact the authenticator service")
	flag.Parse()
	return &f
}
