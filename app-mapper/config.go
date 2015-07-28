package main

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Config struct {
	LogLevel                 string `yaml:"log-level"`
	MapperType               string `yaml:"mapper-type"`
	ConstantMapperTargetHost string `yaml:"constant-mapper-target-host"`
	AppMapperDBHost          string `yaml:"app-mapper-db-host"`
	AuthenticatorType        string `yaml:"authenticator-type"`
	AuthenticatorHost        string `yaml:"authenticator-host"`
}

func ParseConfig(path string) (*Config, error) {
	c := Config{}
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(raw, &c); err != nil {
		return nil, err
	}

	return &c, nil
}
