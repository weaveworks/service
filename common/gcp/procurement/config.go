package procurement

import (
	"flag"
	"time"
)

// Config for the Partner Procurement API
type Config struct {
	Timeout               time.Duration
	ServiceAccountKeyFile string
	ProviderID            string
}

// RegisterFlags sets up config for the Partner Subscriptions API.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	name := "partner-procurement-api"
	flag.DurationVar(&cfg.Timeout, name+".timeout", 10*time.Second, "HTTP client timeout")
	flag.StringVar(&cfg.ServiceAccountKeyFile, name+".service-account-key-file", "", "Service account key JSON file")
	flag.StringVar(&cfg.ProviderID, name+".provider-id", "weaveworks-public", "The GCP project id where the solution resides")
}
