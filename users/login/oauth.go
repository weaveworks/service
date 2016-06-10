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
	Name string
	oauth2.Config
}

// Oauth providers will need to store some session data for each user.
type oauthUserSession struct {
	Token *oauth2.Token `json:"token"`
}

// Flags sets the flags this provider requires on the command-line
func (a *OAuth) Flags(flags *flag.FlagSet) {
	name := strings.ToLower(a.Name)
	flag.StringVar(&a.Config.ClientID, name+"-client-id", a.Config.ClientID, "The application's ID for "+a.Name+" OAuth")
	flag.StringVar(&a.Config.ClientSecret, name+"-client-secret", a.Config.ClientSecret, "The application's secret for "+a.Name+" OAuth")
	flag.StringVar(&a.Config.Endpoint.AuthURL, name+"-auth-url", a.Config.Endpoint.AuthURL, "The URL to redirect users to begin the "+a.Name+" OAuth flow")
	flag.StringVar(&a.Config.Endpoint.TokenURL, name+"-token-url", a.Config.Endpoint.TokenURL, "The URL request "+a.Name+" OAuth tokens from")
	flag.StringVar(&a.Config.RedirectURL, name+"-redirect-url", a.Config.RedirectURL, "The URL to redirect users after going through the "+a.Name+" OAuth flow")
}

// Link is a map of attributes for a link rendered into the UI. When the user
// clicks it, it kicks off the remote authorization flow.
func (a *OAuth) Link(id string, r *http.Request) map[string]string {
	return map[string]string{
		"href": a.Config.AuthCodeURL(
			a.encodeState(map[string]string{
				"token": nosurf.Token(r),
			}),
		),
		"label": "Log in with " + a.Name,
	}
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
	return nosurf.VerifyToken(nosurf.Token(r), state["token"])
}
