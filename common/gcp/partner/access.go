package partner

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/weaveworks/service/common/gcp"
	"github.com/weaveworks/service/users/login"
)

// Accessor describes public methods of Access to allow mocking.
type Accessor interface {
	RequestSubscription(ctx context.Context, token *oauth2.Token, name string) (*Subscription, error)
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
	l.SetName("Partner Subscriptions API")
	return l
}

// Link provides the href for initiating the oauth2 flow.
func (a *Access) Link(r *http.Request) (login.Link, bool) {
	l, ok := a.OAuth.Link(r)
	return login.Link{Href: l.Href}, ok
}

// RequestSubscription fetches a subscription using the user's oauth2 token.
func (a *Access) RequestSubscription(ctx context.Context, token *oauth2.Token, name string) (*Subscription, error) {
	cl, err := NewClientFromTokenSource(oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, err
	}
	return cl.GetSubscription(ctx, name)
}
