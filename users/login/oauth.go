package login

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/justinas/nosurf"
	"golang.org/x/oauth2"
	"net/url"
)

// OAuth authenticates users via generic oauth. It should probably be embedded
// in another type, or at least needs the Config set.
type OAuth struct {
	name string
	oauth2.Config

	// RedirectFromRequest alters the RedirectURL based on the incoming HTTP request
	// used for local development only
	RedirectFromRequest bool
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
	flag.BoolVar(&a.RedirectFromRequest, name+"-redirect-from-request", a.RedirectFromRequest, "Rewrite the hostname & scheme of the redirect URL from the incoming HTTP request (Development only)")
}

// oauthConfig allows us to be more permissive with oauth redirects in local development
func (a *OAuth) oauthConfig(r *http.Request) oauth2.Config {
	if a.RedirectFromRequest {
		referer, refererParseErr := url.ParseRequestURI(r.Referer())
		redirURL, redirectParseErr := url.ParseRequestURI(a.Config.RedirectURL)
		if refererParseErr == nil && redirectParseErr == nil {
			redirURL.Host = referer.Host
			redirURL.Scheme = referer.Scheme

			return oauth2.Config{
				ClientID:     a.Config.ClientID,
				ClientSecret: a.Config.ClientSecret,
				Endpoint:     a.Config.Endpoint,
				Scopes:       a.Config.Scopes,

				RedirectURL: redirURL.String(),
			}
		}
	}
	return a.Config
}

// Link is a map of attributes for a link rendered into the UI. When the user
// clicks it, it kicks off the remote authorization flow.
func (a *OAuth) Link(r *http.Request) (Link, bool) {
	token := csrfToken(r)
	if token == "" {
		// Do not allow linking accounts if the anti CSRF token isn't set
		return Link{}, false
	}
	state := make(map[string]string)
	// pass on any query parameters (ignoring duplicate keys)
	for key, values := range r.URL.Query() {
		state[key] = values[0]
	}
	state["token"] = token

	config := a.oauthConfig(r)
	return Link{
		ID: strings.ToLower(a.name),
		Href: config.AuthCodeURL(
			a.encodeState(state),
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

func (a *OAuth) verifyState(r *http.Request) (map[string]string, bool) {
	state, err := a.decodeState(r.FormValue("state"))
	if err != nil {
		return nil, false
	}
	token := state["token"]
	delete(state, "token")
	return state, nosurf.VerifyToken(csrfToken(r), token)
}

// csrfToken extracts the anti-CSRF token from the cookie injected by authfe.
func csrfToken(r *http.Request) string {
	// We cannot simply use nosurf.Token() since it assumes that the CSRFHandler is being invoked
	// and nosurf doesn't provide another way to extract the Token directly, so we hack
	// our way around by invoking a phony handler.
	var token string
	handler := func(w http.ResponseWriter, r *http.Request) {
		token = nosurf.Token(r)
	}
	phonyCSRFHandler := nosurf.New(http.HandlerFunc(handler))
	phonyCSRFHandler.ExemptFunc(func(r *http.Request) bool { return true })
	phonyResposeWriter := httptest.NewRecorder()
	phonyCSRFHandler.ServeHTTP(phonyResposeWriter, r)
	return token
}
