package partner

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/weaveworks/service/users/login"
)

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
				Scopes:   []string{"https://www.googleapis.com/auth/cloud-billing-partner-subscriptions.readonly"},
			},
		},
	}
	l.SetName("Partner Subscriptions API")
	return l
}

// Link provides the href for initiating the oauth2 flow.
func (a *Access) Link(r *http.Request) (login.Link, bool) {
	l, ok := a.OAuth.Link(r)
	return login.Link{Href: l.Href}, ok
}

// RequestSubscription fetches a subscription. It uses the provided oauth2 code
// to access and download the subscription info.
func (a *Access) RequestSubscription(ctx context.Context, r *http.Request, name string) (*Subscription, error) {
	// Request a token from the oauth2 code
	tok, err := a.Config.Exchange(ctx, r.FormValue("code"))
	if err != nil {
		return nil, err
	}

	// Now verify that the user's token can actually access the subscription
	cl, err := NewClientFromTokenSource(oauth2.StaticTokenSource(tok))
	if err != nil {
		return nil, err
	}
	return cl.GetSubscription(ctx, name)
}
