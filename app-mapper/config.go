package main

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type config struct {
	logLevel                 string `yaml:"log-level"`
	mapperType               string `yaml:"mapper-type"`
	constantMapperTargetHost string `yaml:"constant-mapper-target-host"`
	appMapperDBHost          string `yaml:"app-mapper-db-host"`
	authenticatorType        string `yaml:"authenticator-type"`
	authenticatorHost        string `yaml:"authenticator-host"`
}

func parseConfig(path string) (*config, error) {
	c := config{}
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(raw, &c); err != nil {
		return nil, err
	}

	return &c, nil
}
