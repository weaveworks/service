// FIXME(rndstr): is this actually needed now?
package procurement

import (
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/weaveworks/service/common/gcp"
	"github.com/weaveworks/service/users/login"
)

// Accessor describes public methods of Access to allow mocking.
type Accessor interface {
	VerifyState(r *http.Request) (map[string]string, bool)
	Link(r *http.Request) (login.Link, bool)
}

// Access provides a subscription permission check.
// It uses oauth2 to verify a user can access a specific subscription.
type Access struct {
	login.OAuth
}

// NewAccess creates the oauth instance.
func NewAccess() *Access {
	l := &Access{
		OAuth: login.OAuth{
			Config: oauth2.Config{
				Endpoint: google.Endpoint,
				Scopes:   []string{gcp.OAuthScopeCloudBillingPartnerSubscriptionsRO},
			},
		},
	}
	// This name determines the prefix for the CLI config. (see login.OAuth.Flags())
	// Do not change this or CLI flag names will change.
	l.SetName("Partner Procurement API")
	return l
}

// Link provides the href for initiating the oauth2 flow.
func (a *Access) Link(r *http.Request) (login.Link, bool) {
	l, ok := a.OAuth.Link(r)
	return login.Link{Href: l.Href}, ok
}