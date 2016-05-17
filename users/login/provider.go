package login

import (
	"encoding/json"
	"flag"
	"net/http"
	"sort"
	"sync"
)

// Provider is the interface that login providers fulfill
type Provider interface {
	// Flags sets the flags this provider requires on the command-line
	Flags(*flag.FlagSet)

	// URL we should redirect the user to for this provider. If an ID is
	// provided, we'll pass that through the redirect and associate the resultant
	// account with it.
	// Note: Providers which do not have an associated link, should return nil.
	Link(id string, r *http.Request) map[string]string

	// Login handles a user login request with this provider. It should return
	// the remote ID, email, and session for a user.
	Login(r *http.Request) (id, email string, session json.RawMessage, err error)

	// Username fetches a user's username on the remote service, for displaying
	// *which* account this is linked with.
	Username(session json.RawMessage) (string, error)
}

var (
	mtx       = sync.Mutex{}
	providers = map[string]Provider{}
)

func init() {
	Register("github", Github())
	Register("google", Google())
}

// Flags sets up the command-line flags for each registered provider.
func Flags(flags *flag.FlagSet) {
	ForEach(func(_ string, p Provider) {
		p.Flags(flags)
	})
}

// Get gets an provider by its id.
func Get(id string) (Provider, bool) {
	mtx.Lock()
	a, ok := providers[id]
	mtx.Unlock()
	return a, ok
}

// Register registers a new provider by its id.
func Register(id string, p Provider) {
	mtx.Lock()
	providers[id] = p
	mtx.Unlock()
}

// ForEach calls f for each provider.
func ForEach(f func(string, Provider)) {
	mtx.Lock()
	keys := []string{}
	for id := range providers {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		f(id, providers[id])
	}
	mtx.Unlock()
}

// Reset deletes all the registered providers.
func Reset() {
	mtx.Lock()
	providers = map[string]Provider{}
	mtx.Unlock()
}
