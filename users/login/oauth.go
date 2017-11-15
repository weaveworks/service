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
	"github.com/weaveworks/service/common"
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

// SetName changes the assigned name of this oauth2 endpoint.
func (a *OAuth) SetName(name string) {
	a.name = name
}

// Flags sets the flags this provider requires on the command-line
func (a *OAuth) Flags(flags *flag.FlagSet) {
	name := strings.Replace(strings.ToLower(a.name), " ", "-", -1)
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
	// pass on any query parameters (ignoring duplicate keys)
	state := common.FlattenQueryParams(r.URL.Query())
	state["token"] = token

	return Link{
		ID: strings.ToLower(a.name),
		Href: a.Config.AuthCodeURL(
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

// VerifyState validates the token found within the state query param.
func (a *OAuth) VerifyState(r *http.Request) (map[string]string, bool) {
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
	phonyResponseWriter := httptest.NewRecorder()
	phonyCSRFHandler.ServeHTTP(phonyResponseWriter, r)
	return token
}
