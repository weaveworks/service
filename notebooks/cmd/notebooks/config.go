package main

import (
	"flag"
	"time"
)

// Config for the notebooks service
type Config struct {
	usersServiceType     string
	usersServiceURL      string
	usersCacheSize       int
	usersCacheExpiration time.Duration
}

// RegisterFlags for the notebooks service
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	flag.StringVar(&cfg.usersServiceType, "users-service", "grpc", "What service to use: grpc | mock")
	flag.StringVar(&cfg.usersServiceURL, "users-service.url", "users:4772", "Where to find the users service")
}
