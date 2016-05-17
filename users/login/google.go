package login

import (
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	googleOauth "golang.org/x/oauth2/google"
	plus "google.golang.org/api/plus/v1"
)

type google struct {
	OAuth
}

// Google authenticates users via github oauth
func Google() Provider {
	return &google{
		OAuth: OAuth{
			Name: "Google",
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

func (g *google) Link(id string, r *http.Request) map[string]string {
	return map[string]string{
		"href":            g.OAuth.Link(id, r)["href"],
		"label":           "Google",
		"id":              "google",
		"name":            "Google",
		"icon":            "fa fa-google",
		"backgroundColor": "#dd4b39",
	}
}

// Login converts a user to a db ID
func (g *google) Login(r *http.Request) (string, string, json.RawMessage, error) {
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

	person, err := g.person(tok)
	if err != nil {
		return "", "", nil, err
	}

	email, err := g.personEmail(person)
	if err != nil {
		return "", "", nil, err
	}

	session, err := json.Marshal(oauthUserSession{Token: tok})
	return person.Id, email, session, err
}

// Username fetches a user's username on the remote service, for displaying *which* account this is linked with.
func (g *google) Username(session json.RawMessage) (string, error) {
	var s oauthUserSession
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
