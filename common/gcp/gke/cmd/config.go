package gke

import (
	"flag"
)

// Config for the Kubernetes Engine API
type Config struct {
	ServiceAccountKeyFile string // We need an OAuth2 client
}

// RegisterFlags sets up config for the Partner Subscriptions API.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	name := "gke-api"
	flag.StringVar(&cfg.ServiceAccountKeyFile, name+".oauth.service-account-key-file", "", "Service account key JSON file")
}
