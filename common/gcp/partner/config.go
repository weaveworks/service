package partner

import (
	"flag"
	"time"
)

// Config for the Partner Subscriptions API
type Config struct {
	URL                   string
	Timeout               time.Duration
	ServiceAccountKeyFile string
}

// RegisterFlags sets up config for the Partner Subscriptions API.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	name := "partner-subscriptions-api"
	flag.StringVar(&cfg.URL, name+".url", "https://cloudbilling.googleapis.com/v1", "The base URL of the Google Partner Subscriptions API (including version path)")
	flag.DurationVar(&cfg.Timeout, name+".timeout", 10*time.Second, "HTTP client timeout")
	flag.StringVar(&cfg.ServiceAccountKeyFile, name+".service-account-key-file", "", "Service account key JSON file")
}
