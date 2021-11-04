package login

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc"
	"github.com/justinas/nosurf"
	"golang.org/x/oauth2"
	"gopkg.in/auth0.v5/management"

	"github.com/weaveworks/service/common"
)

var errOAuthStateMismatch = errors.New("oauth state value did not match")
var errOidcNoIDToken = errors.New("no id_token in OIDC response")

// OIDCUserSession stores the OAuth provider's session for a given user.
// This is stored in the database for the login
type OIDCUserSession struct {
	Token   *oauth2.Token `json:"token"`
	IDToken string        `json:"id_token"`
}

// Connection is a representation of a Auth0 side federated login connection
// e.g. google/github
type Connection struct {
	Scopes         []string
	ConnectionName string
}

// Auth0Provider is the main type for interactions with Auth0
type Auth0Provider struct {
	// The domain that handles our auth - e.g. the Auth0 domain we use
	authDomain *url.URL
	// The domain of this Weave Cloud instance
	siteDomain   *url.URL
	configs      map[string]Connection
	provider     *oidc.Provider
	api          *management.Management
	clientID     string
	clientSecret string
}

// NewAuth0Provider starts to initialize a new Auth0 provider. This
// will need to be registered, have flags parsed, and a site domain
// set, before it's ready to use
func NewAuth0Provider() Auth0Provider {
	return Auth0Provider{configs: make(map[string]Connection)}
}

// Register registers a new provider by its id.
func (p *Auth0Provider) Register(id string, c Connection) {
	p.configs[id] = c
}

// Flags registers global authentication flags
func (p *Auth0Provider) Flags(flags *flag.FlagSet) {
	flag.Func("oidc-endpoint-url", "The OIDC endpoint", p.setEndpoint)
	flag.StringVar(&p.clientID, "oidc-client-id", p.clientID, "The application's ID for Auth0")
	flag.StringVar(&p.clientSecret, "oidc-client-secret", p.clientSecret, "The application's secret for Auth0")
}

func (p *Auth0Provider) setEndpoint(endpoint string) error {
	provider, err := oidc.NewProvider(context.Background(), endpoint)
	if err != nil {
		return err
	}
	p.provider = provider
	p.authDomain, err = url.Parse(endpoint)
	return err
}

// SetSiteDomain sets the domain for this installation, e.g. cloud.weave.works
func (p *Auth0Provider) SetSiteDomain(siteDomain string) error {
	var err error
	if p.siteDomain, err = url.Parse(siteDomain); err != nil {
		return err
	}
	// Hack: This runs after flag parsing, so all these parameters should be set already
	p.api, err = NewAuth0Management(p.authDomain.Host, p.clientID, p.clientSecret)
	return err
}

// OAuthConfig creates an oauth2 config object
// If a connection is specified, that connection's scopes will be requested
func (p *Auth0Provider) OAuthConfig(connection *string) oauth2.Config {
	returnURL, _ := p.siteDomain.Parse("/login-via/auth0")
	c := oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		Endpoint:     p.provider.Endpoint(),
		RedirectURL:  returnURL.String(),
	}
	if connection != nil {
		c.Scopes = p.configs[*connection].Scopes
	}
	return c
}

// LoginURL constructs the URL to send user to when they want to login.
// This specifies which connection to use, so they should go straight to
// e.g. google/github
func (p *Auth0Provider) LoginURL(r *http.Request, connection string) string {
	token := csrfToken(r)

	// pass on any query parameters (ignoring duplicate keys)
	state := common.FlattenQueryParams(r.URL.Query())
	state["token"] = token
	urlParam := oauth2.SetAuthURLParam("connection", p.configs[connection].ConnectionName)

	config := p.OAuthConfig(&connection)
	return config.AuthCodeURL(p.encodeState(state), urlParam)
}

// LogoutURL constructs the URL to use to log out
func (p *Auth0Provider) LogoutURL(r *http.Request) string {
	returnURL, _ := p.siteDomain.Parse("/login")
	logoutURL := *p.authDomain
	logoutURL.Path = "v2/logout"
	logoutURL.RawQuery = url.Values{"client_id": {p.clientID}, "returnTo": {returnURL.String()}}.Encode()
	return logoutURL.String()
}

func (p *Auth0Provider) extractClaims(ctx context.Context, rawIDToken string) (*Claims, error) {
	verifier := p.provider.Verifier(&oidc.Config{ClientID: p.clientID})

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}

	var claims Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, err
	}

	return &claims, nil
}

// Login is called after the user has authenticated with auth0. It turns that
// request into a set of claims
func (p *Auth0Provider) Login(r *http.Request) (*Claims, json.RawMessage, map[string]string, error) {
	extraState, ok := p.VerifyState(r)
	if !ok {
		return nil, nil, nil, errOAuthStateMismatch
	}
	config := p.OAuthConfig(nil)

	// Use the authorization code that is pushed to the redirect URL.
	// NewTransportWithCode will do the handshake to retrieve
	// an access token and initiate a Transport that is
	// authorized and authenticated by the retrieved token.
	tok, err := config.Exchange(r.Context(), r.FormValue("code"))
	if err != nil {
		return nil, nil, nil, err
	}

	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok {
		return nil, nil, nil, errOidcNoIDToken
	}

	claims, err := p.extractClaims(r.Context(), rawIDToken)
	if err != nil {
		return nil, nil, nil, err
	}

	session, err := json.Marshal(OIDCUserSession{Token: tok, IDToken: rawIDToken})
	return claims, session, extraState, err
}

// State returns a tamper-proof (but not encrypted), hmac-ed state string, including:
// * a csrf token to match with the user's cookie
// * maybe a uri to redirect the user to once logged in
func (p *Auth0Provider) encodeState(raw map[string]string) string {
	// TODO: handle this error, for now we'll just generate an invalid state.
	j, _ := json.Marshal(raw)
	sum := hmac.New(sha256.New, []byte(p.clientSecret)).Sum(j)
	return fmt.Sprintf(
		"%s:%s",
		base64.RawURLEncoding.EncodeToString(j),
		base64.RawURLEncoding.EncodeToString(sum),
	)
}

func (p *Auth0Provider) decodeState(raw string) (map[string]string, error) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return nil, errOAuthStateMismatch
	}

	j, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errOAuthStateMismatch
	}
	sum, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errOAuthStateMismatch
	}

	expected := hmac.New(sha256.New, []byte(p.clientSecret)).Sum(j)
	if !hmac.Equal(expected, sum) {
		return nil, errOAuthStateMismatch
	}

	var m map[string]string
	return m, json.Unmarshal(j, &m)
}

// VerifyState validates the token found within the state query param.
func (p *Auth0Provider) VerifyState(r *http.Request) (map[string]string, bool) {
	state, err := p.decodeState(r.FormValue("state"))
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
