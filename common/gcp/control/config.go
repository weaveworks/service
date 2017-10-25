package control

import (
	"flag"
	"time"
)

// Config for the Google Service Control API
type Config struct {
	Timeout               time.Duration
	ServiceAccountKeyFile string
}

// RegisterFlags sets up config for the Service Control API.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	name := "service-control-api"
	flag.DurationVar(&cfg.Timeout, name+".timeout", 10*time.Second, "HTTP client timeout")
	flag.StringVar(&cfg.ServiceAccountKeyFile, name+".service-account-key-file", "", "Service account key JSON file")
}
