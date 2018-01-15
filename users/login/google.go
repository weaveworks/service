package login

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/common/gcp"
	"github.com/weaveworks/service/common/gcp/gke"
	"golang.org/x/oauth2"
	googleOauth "golang.org/x/oauth2/google"
	plus "google.golang.org/api/plus/v1"
)

// GoogleProviderID is the ID to register this login provider.
const GoogleProviderID = "google"

type google struct {
	OAuth
}

// NewGoogleProvider authenticates users via google oauth
func NewGoogleProvider() Provider {
	return &google{
		OAuth: OAuth{
			name: "Google",
			Config: oauth2.Config{
				Endpoint: googleOauth.Endpoint,
				Scopes: []string{
					"https://www.googleapis.com/auth/plus.me",
					"https://www.googleapis.com/auth/userinfo.email",
					"https://www.googleapis.com/auth/userinfo.profile",
				},
			},
		},
	}
}

func (g *google) Link(r *http.Request) (Link, bool) {
	l, ok := g.OAuth.Link(r)
	// If user is subscribing from GCP Cloud Launcher,
	// we request one additional scope to be able to access his subscriptions,
	// in order to be able to bill her via GCP and manage access to Weave Cloud.
	if isSubscribingFromGCP(r) {
		l.Href = addGCPSubscriptionScope(l.Href)
	}
	l.BackgroundColor = "#dd4b39"
	return l, ok
}

func isSubscribingFromGCP(r *http.Request) bool {
	// N.B.: when user is subscribing from GCP, the only gcpAccountId is provided
	// as a query parameter. When user is SSO-ing from GCP, both are provided.
	gcpAccountID := r.URL.Query().Get("gcpAccountId")
	ssoToken := r.URL.Query().Get("ssoToken")
	return gcpAccountID != "" && ssoToken == ""
}

func addGCPSubscriptionScope(oauthURL string) string {
	u, err := url.Parse(oauthURL)
	if err != nil {
		log.Errorf("Failed to parse Google OAuth URL, falling back to existing URL. Root cause: %v", err)
		return oauthURL
	}
	q := u.Query()
	q.Set("scope", strings.Join([]string{
		q.Get("scope"),
		gcp.OAuthScopeCloudBillingPartnerSubscriptionsRO,
		gke.OAuthScopeCloudPlatform,
	}, " "))
	u.RawQuery = q.Encode()
	return u.String()
}

// Login converts a user to a db ID
func (g *google) Login(r *http.Request) (string, string, json.RawMessage, map[string]string, error) {
	extraState, ok := g.VerifyState(r)
	if !ok {
		return "", "", nil, nil, errOAuthStateMismatch
	}

	// Use the authorization code that is pushed to the redirect URL.
	// NewTransportWithCode will do the handshake to retrieve
	// an access token and initiate a Transport that is
	// authorized and authenticated by the retrieved token.
	tok, err := g.Config.Exchange(context.TODO(), r.FormValue("code"))
	if err != nil {
		return "", "", nil, nil, err
	}

	person, err := g.person(tok)
	if err != nil {
		return "", "", nil, nil, err
	}

	email, err := g.personEmail(person)
	if err != nil {
		return "", "", nil, nil, err
	}

	session, err := json.Marshal(OAuthUserSession{Token: tok})
	return person.Id, email, session, extraState, err
}

// Username fetches a user's username on the remote service, for displaying *which* account this is linked with.
func (g *google) Username(session json.RawMessage) (string, error) {
	var s OAuthUserSession
	if err := json.Unmarshal(session, &s); err != nil {
		return "", err
	}
	person, err := g.person(s.Token)
	if err != nil {
		return "", err
	}
	return g.personEmail(person)
}

func (g *google) person(token *oauth2.Token) (*plus.Person, error) {
	oauthClient := g.Config.Client(oauth2.NoContext, token)
	plusService, err := plus.New(oauthClient)
	if err != nil {
		return nil, err
	}
	return plus.NewPeopleService(plusService).Get("me").Do()
}

func (g *google) personEmail(p *plus.Person) (string, error) {
	for _, e := range p.Emails {
		if e.Type == "account" {
			return e.Value, nil
		}
	}
	return "", fmt.Errorf("Invalid authentication data")
}

// Logout handles a user logout request with this provider. It should revoke
// the remote user session, requiring the user to re-authenticate next time.
func (g *google) Logout(session json.RawMessage) error {
	var s OAuthUserSession
	if err := json.Unmarshal(session, &s); err != nil {
		return err
	}

	response, err := http.Get(fmt.Sprintf("https://accounts.google.com/o/oauth2/revoke?token=%s", s.Token.AccessToken))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusBadRequest:
		// Ignore bad requests here, as we'll just assume the revocation was successful.
		body, _ := ioutil.ReadAll(response.Body)
		log.Warningf("Error revoking google oauth token: %s %q", response.Status, body)
		return nil
	default:
		body, _ := ioutil.ReadAll(response.Body)
		log.Warningf("Error revoking google oauth token: %s %q", response.Status, body)
	}
	return fmt.Errorf("Error revoking google oauth token: %s", response.Status)
}
