package control

import (
	"flag"
	"time"
)

// Config for the Google Service Control API
type Config struct {
	URL                   string
	ServiceAccountKeyFile string
	ServiceName           string
	Timeout               time.Duration
}

// RegisterFlags sets up config for the Service Control API.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	name := "service-control-api"
	// Staging: https://staging-servicecontrol.sandbox.googleapis.com
	flag.StringVar(&cfg.URL, name+".url", "https://servicecontrol.googleapis.com", "Base path of the API")
	flag.StringVar(&cfg.ServiceAccountKeyFile, name+".service-account-key-file", "", "Service account key JSON file")
	// Staging: staging.google.weave.works
	flag.StringVar(&cfg.ServiceName, name+".service-name", "google.weave.works", "API service name")
	flag.DurationVar(&cfg.Timeout, name+".timeout", 10*time.Second, "HTTP client timeout")
}
