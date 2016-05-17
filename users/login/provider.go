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

	// Name is the human-readable name of this provider
	Name() string

	// Link is a map of attributes for a link rendered into the UI. When the user
	// clicks it, it kicks off the remote authorization flow.
	// Note: Providers which do not have an associated link, should return false.
	Link(r *http.Request) (Link, bool)

	// Login handles a user login request with this provider. It should return
	// the remote ID, email, and session for a user.
	Login(r *http.Request) (id, email string, session json.RawMessage, err error)

	// Username fetches a user's username on the remote service, for displaying
	// *which* account this is linked with.
	Username(session json.RawMessage) (string, error)
}

// Link is the attributes of a rendered HTML button the user should click to
// start the login flow.
type Link struct {
	ID              string `json:"id,omitempty"`              // HTML ID for this button
	Href            string `json:"href,omitempty"`            // URL this button should take the user to
	Label           string `json:"label,omitempty"`           // Human-readable text for the rendered button
	Icon            string `json:"icon,omitempty"`            // Icon class, for the rendered button (probably a font-awesome icon class)
	BackgroundColor string `json:"backgroundColor,omitempty"` // Hex background colour for the rendered button
}

// Providers is a registry set of login providers
type Providers struct {
	mtx       sync.Mutex
	providers map[string]Provider
}

// NewProviders creates a new provider registry
func NewProviders() *Providers {
	return &Providers{providers: map[string]Provider{}}
}

// Flags sets up the command-line flags for each registered provider.
func (ps *Providers) Flags(flags *flag.FlagSet) {
	ps.ForEach(func(_ string, p Provider) {
		p.Flags(flags)
	})
}

// Get gets an provider by its id.
func (ps *Providers) Get(id string) (Provider, bool) {
	ps.mtx.Lock()
	a, ok := ps.providers[id]
	ps.mtx.Unlock()
	return a, ok
}

// Register registers a new provider by its id.
func (ps *Providers) Register(id string, p Provider) {
	ps.mtx.Lock()
	ps.providers[id] = p
	ps.mtx.Unlock()
}

// ForEach calls f for each provider.
func (ps *Providers) ForEach(f func(string, Provider)) {
	ps.mtx.Lock()
	keys := []string{}
	for id := range ps.providers {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		f(id, ps.providers[id])
	}
	ps.mtx.Unlock()
}

// Reset deletes all the registered providers.
func (ps *Providers) Reset() {
	ps.mtx.Lock()
	ps.providers = map[string]Provider{}
	ps.mtx.Unlock()
}
