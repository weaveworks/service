package login

import (
	"encoding/json"
	"fmt"
	"net/http"

	gClient "github.com/google/go-github/github"
	"golang.org/x/oauth2"
	githubOauth "golang.org/x/oauth2/github"
)

type github struct {
	OAuth
}

// NewGithubProvider authenticates users via github oauth
func NewGithubProvider() Provider {
	return &github{
		OAuth: OAuth{
			name: "Github",
			Config: oauth2.Config{
				Endpoint: githubOauth.Endpoint,
				Scopes:   []string{"user:email", "repo", "write:public_key", "read:public_key"},
			},
		},
	}
}

func (g *github) Link(r *http.Request) (Link, bool) {
	l, _ := g.OAuth.Link(r)
	l.BackgroundColor = "#444444"
	return l, true
}

// Login converts a user to a db ID
func (g *github) Login(r *http.Request) (string, string, json.RawMessage, error) {
	if !g.verifyState(r) {
		return "", "", nil, fmt.Errorf("oauth state value did not match")
	}

	// Use the authorization code that is pushed to the redirect URL.
	// NewTransportWithCode will do the handshake to retrieve
	// an access token and initiate a Transport that is
	// authorized and authenticated by the retrieved token.
	tok, err := g.Config.Exchange(oauth2.NoContext, r.FormValue("code"))
	if err != nil {
		return "", "", nil, err
	}

	oauthClient := g.Config.Client(oauth2.NoContext, tok)
	client := gClient.NewClient(oauthClient)
	user, _, err := client.Users.Get("")
	if err != nil {
		return "", "", nil, err
	}
	emails, _, err := client.Users.ListEmails(nil)
	if err != nil {
		return "", "", nil, err
	}
	var email string
	for _, e := range emails {
		if *e.Primary && *e.Verified {
			email = *e.Email
		}
	}

	session, err := json.Marshal(oauthUserSession{Token: tok})
	return fmt.Sprint(*user.ID), email, session, err
}

// Username fetches a user's username on the remote service, for displaying *which* account this is linked with.
func (g *github) Username(session json.RawMessage) (string, error) {
	var s oauthUserSession
	if err := json.Unmarshal(session, &s); err != nil {
		return "", err
	}
	oauthClient := g.Config.Client(oauth2.NoContext, s.Token)
	client := gClient.NewClient(oauthClient)
	user, _, err := client.Users.Get("")
	if err != nil {
		return "", err
	}
	return *user.Login, nil
}

// Logout handles a user logout request with this provider. It should revoke
// the remote user session, requiring the user to re-authenticate next time.
func (g *github) Logout(session json.RawMessage) error {
	var s oauthUserSession
	if err := json.Unmarshal(session, &s); err != nil {
		return err
	}
	client := gClient.NewClient(&http.Client{
		Transport: &basicAuthTransport{
			username: g.Config.ClientID,
			password: g.Config.ClientSecret,
		},
	})
	response, err := client.Authorizations.Revoke(g.Config.ClientID, s.Token.AccessToken)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		if response == nil || response.StatusCode != http.StatusNotFound {
			return err
		}
	}
	return nil
}

type basicAuthTransport struct {
	username, password string
	http.RoundTripper
}

func (c *basicAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if c.RoundTripper == nil {
		c.RoundTripper = http.DefaultTransport
	}
	r.SetBasicAuth(c.username, c.password)
	return c.RoundTripper.RoundTrip(r)
}
