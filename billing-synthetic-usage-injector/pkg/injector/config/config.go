package config

import (
	"flag"
	"time"
)

// Flags mimics the CLI arguments & options passed in to Scope.
type Flags struct {
	Scope    ScopeFlags
	NumHosts uint
}

// ScopeFlags mimics the CLI arguments & options passed in to Scope probes.
type ScopeFlags struct {
	URL             string
	Token           string
	PublishInterval time.Duration
	SpyInterval     time.Duration
	Insecure        bool
	NoControls      bool
}

// RegisterFlags associates CLI arguments & options with this Flags instance's fields.
func (flags *Flags) RegisterFlags(f *flag.FlagSet) {
	f.UintVar(&flags.NumHosts, "num-hosts", 1, "Number of hosts to report fake usage for.")
	// Scope-specific flags:
	f.StringVar(&flags.Scope.URL, "scope.url", "https://frontend.dev.weave.works.:443", "URL to connect Weave Cloud")
	f.StringVar(&flags.Scope.Token, "scope.token", "", "Token to authenticate with Weave Cloud")
	f.DurationVar(&flags.Scope.PublishInterval, "scope.publish.interval", 3*time.Second, "publish (output) interval")
	f.DurationVar(&flags.Scope.SpyInterval, "scope.spy.interval", time.Second, "spy (scan) interval")
	f.BoolVar(&flags.Scope.NoControls, "scope.no-controls", false, "Disable controls (e.g. start/stop containers, terminals, logs ...)")
	f.BoolVar(&flags.Scope.Insecure, "scope.insecure", false, "(SSL) explicitly allow \"insecure\" SSL connections and transfers")
}
