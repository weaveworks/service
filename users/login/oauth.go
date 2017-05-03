package login

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/justinas/nosurf"
	"golang.org/x/oauth2"
)

// OAuth authenticates users via generic oauth. It should probably be embedded
// in another type, or at least needs the Config set.
type OAuth struct {
	name string
	oauth2.Config
}

// Oauth providers will need to store some session data for each user.
type oauthUserSession struct {
	Token *oauth2.Token `json:"token"`
}

// Name is the human-readable name of this provider
func (a *OAuth) Name() string {
	return a.name
}

// Flags sets the flags this provider requires on the command-line
func (a *OAuth) Flags(flags *flag.FlagSet) {
	name := strings.ToLower(a.name)
	flag.StringVar(&a.Config.ClientID, name+"-client-id", a.Config.ClientID, "The application's ID for "+a.name+" OAuth")
	flag.StringVar(&a.Config.ClientSecret, name+"-client-secret", a.Config.ClientSecret, "The application's secret for "+a.name+" OAuth")
	flag.StringVar(&a.Config.Endpoint.AuthURL, name+"-auth-url", a.Config.Endpoint.AuthURL, "The URL to redirect users to begin the "+a.name+" OAuth flow")
	flag.StringVar(&a.Config.Endpoint.TokenURL, name+"-token-url", a.Config.Endpoint.TokenURL, "The URL request "+a.name+" OAuth tokens from")
	flag.StringVar(&a.Config.RedirectURL, name+"-redirect-url", a.Config.RedirectURL, "The URL to redirect users after going through the "+a.name+" OAuth flow")
}

// Link is a map of attributes for a link rendered into the UI. When the user
// clicks it, it kicks off the remote authorization flow.
func (a *OAuth) Link(r *http.Request) (Link, bool) {
	token := csrfToken(r)
	if token == "" {
		// Do not allow linking accounts if the anti CSRF token isn't set
		return Link{}, false
	}
	return Link{
		ID: strings.ToLower(a.name),
		Href: a.Config.AuthCodeURL(
			a.encodeState(map[string]string{
				"token": token,
			}),
		),
		Label: a.name,
		Icon:  "fa fa-" + strings.ToLower(a.name),
	}, true
}

// State returns a tamper-proof (but not encrypted), hmac-ed state string, including:
// * a csrf token to match with the user's cookie
// * maybe a uri to redirect the user to once logged in
func (a *OAuth) encodeState(raw map[string]string) string {
	// TODO: handle this error, for now we'll just generate an invalid state.
	j, _ := json.Marshal(raw)
	sum := hmac.New(sha256.New, []byte(a.Config.ClientSecret)).Sum(j)
	return fmt.Sprintf(
		"%s:%s",
		base64.RawURLEncoding.EncodeToString(j),
		base64.RawURLEncoding.EncodeToString(sum),
	)
}

func (a *OAuth) decodeState(raw string) (map[string]string, error) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("oauth state value did not match")
	}

	j, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("oauth state value did not match")
	}
	sum, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oauth state value did not match")
	}

	expected := hmac.New(sha256.New, []byte(a.Config.ClientSecret)).Sum(j)
	if !hmac.Equal(expected, sum) {
		return nil, fmt.Errorf("oauth state value did not match")
	}

	var m map[string]string
	return m, json.Unmarshal(j, &m)
}

func (a *OAuth) verifyState(r *http.Request) bool {
	state, err := a.decodeState(r.FormValue("state"))
	if err != nil {
		return false
	}
	return nosurf.VerifyToken(csrfToken(r), state["token"])
}

// csrfToken extracts the anti-CSRF token from the cookie injected by authfe.
// We cannot simply use nosurf.Token() since it assumes that the CSRFHandler was invoked
func csrfToken(r *http.Request) string {
	tokenCookie, err := r.Cookie(nosurf.CookieName)
	if err != nil {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(tokenCookie.Value)
	if err != nil {
		return ""
	}
	return string(decoded)
}
