package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/bluele/gcache"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common"
)

var (
	authCacheCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: common.PrometheusNamespace,
		Name:      "auth_cache",
		Help:      "Reports fetches that miss local cache.",
	}, []string{"cache", "result"})
)

func init() {
	prometheus.MustRegister(authCacheCounter)
}

// Authenticator is the client interface to the users service.
type Authenticator interface {
	AuthenticateOrg(w http.ResponseWriter, r *http.Request, orgExternalID string) (string, string, []string, error)
	AuthenticateProbe(w http.ResponseWriter, r *http.Request) (string, []string, error)
	AuthenticateAdmin(w http.ResponseWriter, r *http.Request) (string, error)
	AuthenticateUser(w http.ResponseWriter, r *http.Request) (string, error)
}

// Unauthorized is the error type returned when authorisation fails/
type Unauthorized struct {
	httpStatus int
}

func (u Unauthorized) Error() string {
	return http.StatusText(u.httpStatus)
}

// AuthenticatorOptions control behaviour of the authenticator client.
type AuthenticatorOptions struct {
	CredCacheEnabled         bool
	ProbeCredCacheSize       int
	OrgCredCacheSize         int
	ProbeCredCacheExpiration time.Duration
	OrgCredCacheExpiration   time.Duration
}

// MakeAuthenticator is a factory for Authenticators
func MakeAuthenticator(kind, url string, opts AuthenticatorOptions) Authenticator {
	switch kind {
	case "mock":
		return &mockAuthenticator{}
	case "web":
		if opts.CredCacheEnabled {
			return &webAuthenticator{
				url:            url,
				probeCredCache: gcache.New(opts.ProbeCredCacheSize).LRU().Expiration(opts.ProbeCredCacheExpiration).Build(),
				orgCredCache:   gcache.New(opts.OrgCredCacheSize).LRU().Expiration(opts.OrgCredCacheExpiration).Build(),
				client: &http.Client{
					Transport: &nethttp.Transport{
						RoundTripper: &http.Transport{
							// Rest are from http.DefaultTransport
							Proxy: http.ProxyFromEnvironment,
							DialContext: (&net.Dialer{
								Timeout:   30 * time.Second,
								KeepAlive: 30 * time.Second,
							}).DialContext,
							TLSHandshakeTimeout:   10 * time.Second,
							ExpectContinueTimeout: 1 * time.Second,
						},
					},
				},
			}
		}
		return &webAuthenticator{
			url: url,
		}
	default:
		log.Fatal("Incorrect authenticator type: ", kind)
		return nil
	}
}

type mockAuthenticator struct{}

func (mockAuthenticator) AuthenticateOrg(w http.ResponseWriter, r *http.Request, orgExternalID string) (string, string, []string, error) {
	return "mockID", "mockUserID", nil, nil
}

func (mockAuthenticator) AuthenticateProbe(w http.ResponseWriter, r *http.Request) (string, []string, error) {
	return "mockID", nil, nil
}

func (mockAuthenticator) AuthenticateAdmin(w http.ResponseWriter, r *http.Request) (string, error) {
	return "mockID", nil
}

func (mockAuthenticator) AuthenticateUser(w http.ResponseWriter, r *http.Request) (string, error) {
	return "mockID", nil
}

type webAuthenticator struct {
	url            string
	client         *http.Client
	probeCredCache gcache.Cache
	orgCredCache   gcache.Cache
	transport      http.RoundTripper
}

// Constants exported for testing
const (
	AuthCookieName = "_weave_scope_session"
	AuthHeaderName = "Authorization"
)

func hitOrMiss(err error) string {
	if err == nil {
		return "hit"
	}
	return "miss"
}

type orgCredCacheKey struct {
	header, cookie, orgExternalID string
}

type orgCredCacheValue struct {
	orgID, userID string
	featureFlags  []string
}

func (m *webAuthenticator) lookupInOrgCacheOr(key orgCredCacheKey, f func() (string, string, []string, error)) (string, string, []string, error) {
	if m.orgCredCache == nil {
		return f()
	}
	org, err := m.orgCredCache.Get(key)
	authCacheCounter.WithLabelValues("org_cred_cache", hitOrMiss(err)).Inc()
	if err == nil {
		v := org.(orgCredCacheValue)
		return v.orgID, v.userID, v.featureFlags, nil
	}

	orgID, userID, featureFlags, err := f()
	if err == nil {
		m.orgCredCache.Set(key, orgCredCacheValue{
			orgID:        orgID,
			userID:       userID,
			featureFlags: featureFlags,
		})
	}
	return orgID, userID, featureFlags, err
}

func (m *webAuthenticator) AuthenticateOrg(w http.ResponseWriter, r *http.Request, orgExternalID string) (string, string, []string, error) {
	// Extract Authorization header to inject it in the lookup request. If
	// it were not set, don't even bother to do a lookup.
	authHeader, err := getAuthHeader(r, "Scope-User")
	if err == nil {
		return m.authenticateOrgViaHeader(w, r, orgExternalID, authHeader)
	}

	// Extract Authorization cookie to inject it in the lookup request. If it were
	// not set, don't even bother to do a lookup.
	authCookie, err := r.Cookie(AuthCookieName)
	if err == nil {
		return m.authenticateOrgViaCookie(w, r, orgExternalID, authCookie)
	}

	log.Error("authenticator: org: tried to authenticate request without credentials")
	return "", "", nil, &Unauthorized{http.StatusUnauthorized}
}

func (m *webAuthenticator) authenticateOrgViaCookie(w http.ResponseWriter, r *http.Request, orgExternalID string, authCookie *http.Cookie) (string, string, []string, error) {
	return m.lookupInOrgCacheOr(
		orgCredCacheKey{cookie: authCookie.Value, orgExternalID: orgExternalID},
		func() (string, string, []string, error) {
			url := fmt.Sprintf("%s/private/api/users/lookup/%s", m.url, url.QueryEscape(orgExternalID))
			lookupReq, err := http.NewRequest("GET", url, nil)
			if err != nil {
				log.Error("authenticator: cannot build lookup request: ", err)
				return "", "", nil, err
			}
			lookupReq.AddCookie(authCookie)
			lookupReq = lookupReq.WithContext(r.Context())
			return m.decodeOrg(m.doAuthenticateRequest(w, lookupReq))
		},
	)
}

func (m *webAuthenticator) authenticateOrgViaHeader(w http.ResponseWriter, r *http.Request, orgExternalID string, authHeader string) (string, string, []string, error) {
	return m.lookupInOrgCacheOr(
		orgCredCacheKey{header: authHeader, orgExternalID: orgExternalID},
		func() (string, string, []string, error) {
			url := fmt.Sprintf("%s/private/api/users/lookup/%s", m.url, url.QueryEscape(orgExternalID))
			lookupReq, err := http.NewRequest("GET", url, nil)
			if err != nil {
				log.Error("authenticator: cannot build lookup request: ", err)
				return "", "", nil, err
			}
			lookupReq.Header.Set(AuthHeaderName, authHeader)
			lookupReq = lookupReq.WithContext(r.Context())
			return m.decodeOrg(m.doAuthenticateRequest(w, lookupReq))
		},
	)
}

func getAuthHeader(r *http.Request, realm string) (string, error) {
	authHeader := r.Header.Get(AuthHeaderName)
	if strings.HasPrefix(authHeader, realm) {
		return authHeader, nil
	}

	// To allow grafana to talk to the service, we also accept basic auth,
	// ignoring the username and treating the password as the token.
	_, token, ok := r.BasicAuth()
	if ok {
		return fmt.Sprintf("%s token=%s", realm, token), nil
	}

	return "", &Unauthorized{http.StatusUnauthorized}
}

type probeCredCacheValue struct {
	orgID        string
	featureFlags []string
	err          error
}

func (m *webAuthenticator) AuthenticateProbe(w http.ResponseWriter, r *http.Request) (string, []string, error) {
	// Extract Authorization header to inject it in the lookup request. If
	// it were not set, don't even bother to do a lookup.
	authHeader, err := getAuthHeader(r, "Scope-Probe")
	if err != nil {
		log.Error("authenticator: probe: tried to authenticate request without credentials")
		return "", nil, err
	}

	if m.probeCredCache != nil {
		org, err := m.probeCredCache.Get(authHeader)
		authCacheCounter.WithLabelValues("probe_cred_cache", hitOrMiss(err)).Inc()
		if err == nil {
			v := org.(probeCredCacheValue)
			return v.orgID, v.featureFlags, v.err
		}
	}

	url := fmt.Sprintf("%s/private/api/users/lookup", m.url)
	lookupReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("authenticator: cannot build lookup request: ", err)
		return "", nil, err
	}
	lookupReq.Header.Set(AuthHeaderName, authHeader)
	lookupReq = lookupReq.WithContext(r.Context())
	orgID, _, featureFlags, err := m.decodeOrg(m.doAuthenticateRequest(w, lookupReq))
	if m.probeCredCache != nil && (err == nil || isUnauthorized(err)) {
		m.probeCredCache.Set(authHeader, probeCredCacheValue{orgID: orgID, featureFlags: featureFlags, err: err})
	}
	return orgID, featureFlags, err
}

func isUnauthorized(err error) bool {
	unauthorized, ok := err.(*Unauthorized)
	if !ok {
		return false
	}
	return unauthorized.httpStatus == http.StatusUnauthorized
}

func (m *webAuthenticator) AuthenticateAdmin(w http.ResponseWriter, r *http.Request) (string, error) {
	// Extract Authorization cookie to inject it in the lookup request. If it were
	// not set, don't even bother to do a lookup.
	authCookie, err := r.Cookie(AuthCookieName)
	if err != nil {
		log.Error("authenticator: admin: tried to authenticate request without credentials")
		return "", &Unauthorized{http.StatusUnauthorized}
	}

	url := fmt.Sprintf("%s/private/api/users/admin", m.url)
	lookupReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("authenticator: cannot build lookup request: ", err)
		return "", err
	}
	lookupReq.AddCookie(authCookie)
	lookupReq = lookupReq.WithContext(r.Context())
	return m.decodeAdmin(m.doAuthenticateRequest(w, lookupReq))
}

func (m *webAuthenticator) AuthenticateUser(w http.ResponseWriter, r *http.Request) (string, error) {
	// Extract Authorization cookie to inject it in the lookup request. If it were
	// not set, don't even bother to do a lookup.
	authCookie, err := r.Cookie(AuthCookieName)
	if err != nil {
		log.Error("authenticator: admin: tried to authenticate request without credentials")
		return "", &Unauthorized{http.StatusUnauthorized}
	}

	url := fmt.Sprintf("%s/private/api/users/lookup_user", m.url)
	lookupReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("authenticator: cannot build lookup request: ", err)
		return "", err
	}
	lookupReq.AddCookie(authCookie)
	lookupReq = lookupReq.WithContext(r.Context())
	return m.decodeUser(m.doAuthenticateRequest(w, lookupReq))
}

func (m *webAuthenticator) doAuthenticateRequest(w http.ResponseWriter, r *http.Request) (io.ReadCloser, error) {
	var ht *nethttp.Tracer
	r, ht = nethttp.TraceRequest(opentracing.GlobalTracer(), r)
	defer ht.Finish()

	// Contact the authorization server
	res, err := m.client.Do(r)
	if err != nil {
		return nil, err
	}

	// Parse the response
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, &Unauthorized{res.StatusCode}
	}

	// Copy new cookies from the response
	for _, c := range res.Cookies() {
		http.SetCookie(w, c)
	}
	return res.Body, nil
}

func (m *webAuthenticator) decodeOrg(body io.ReadCloser, err error) (string, string, []string, error) {
	if err != nil {
		return "", "", nil, err
	}
	defer body.Close()
	var authRes struct {
		OrganizationID string   `json:"organizationID"`
		UserID         string   `json:"userID"`
		FeatureFlags   []string `json:"featureFlags"`
	}
	if err := json.NewDecoder(body).Decode(&authRes); err != nil {
		return "", "", nil, err
	}
	if authRes.OrganizationID == "" {
		return "", "", nil, errors.New("empty OrganizationID")
	}
	return authRes.OrganizationID, authRes.UserID, authRes.FeatureFlags, nil
}

func (m *webAuthenticator) decodeAdmin(body io.ReadCloser, err error) (string, error) {
	if err != nil {
		return "", err
	}
	defer body.Close()
	var authRes struct {
		AdminID string `json:"adminID"`
	}
	if err := json.NewDecoder(body).Decode(&authRes); err != nil {
		return "", err
	}
	if authRes.AdminID == "" {
		return "", errors.New("empty AdminID")
	}
	return authRes.AdminID, nil
}

func (m *webAuthenticator) decodeUser(body io.ReadCloser, err error) (string, error) {
	if err != nil {
		return "", err
	}
	defer body.Close()
	var authRes struct {
		UserID string `json:"userID"`
	}
	if err := json.NewDecoder(body).Decode(&authRes); err != nil {
		return "", err
	}
	return authRes.UserID, nil
}

// AuthOrgMiddleware is a middleware.Interface for authentication organisations based on the
// cookie and an org name in the path
type AuthOrgMiddleware struct {
	Authenticator       Authenticator
	OrgExternalID       func(*http.Request) (string, bool)
	OutputHeader        string
	UserIDHeader        string
	FeatureFlagsHeader  string
	RequireFeatureFlags []string
}

// Wrap implements middleware.Interface
func (a AuthOrgMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgExternalID, ok := a.OrgExternalID(r)
		if !ok {
			log.Infof("invalid request: %s", r.RequestURI)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		organizationID, userID, featureFlags, err := a.Authenticator.AuthenticateOrg(w, r, orgExternalID)
		if err != nil {
			if unauth, ok := err.(*Unauthorized); ok {
				log.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				log.Errorf("proxy: error contacting authenticator: %v", err)
				w.WriteHeader(http.StatusBadGateway)
			}
			return
		}

		if !hasFeatureAllFlags(a.RequireFeatureFlags, featureFlags) {
			log.Infof("proxy: missing feature flags: %v", a.RequireFeatureFlags)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Header.Add(a.OutputHeader, organizationID)
		r.Header.Add(a.UserIDHeader, userID)
		r.Header.Add(a.FeatureFlagsHeader, strings.Join(featureFlags, " "))
		next.ServeHTTP(w, r.WithContext(user.WithID(r.Context(), organizationID)))
	})
}

// AuthProbeMiddleware is a middleware.Interface for authentication probes based on the headers
type AuthProbeMiddleware struct {
	Authenticator       Authenticator
	OutputHeader        string
	FeatureFlagsHeader  string
	RequireFeatureFlags []string
}

// Wrap implements middleware.Interface
func (a AuthProbeMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		organizationID, featureFlags, err := a.Authenticator.AuthenticateProbe(w, r)
		if err != nil {
			if unauth, ok := err.(*Unauthorized); ok {
				log.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				log.Errorf("proxy: error contacting authenticator: %v", err)
				w.WriteHeader(http.StatusBadGateway)
			}
			return
		}

		if !hasFeatureAllFlags(a.RequireFeatureFlags, featureFlags) {
			log.Infof("proxy: missing feature flags: %v", a.RequireFeatureFlags)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Header.Add(a.OutputHeader, organizationID)
		r.Header.Add(a.FeatureFlagsHeader, strings.Join(featureFlags, " "))
		next.ServeHTTP(w, r.WithContext(user.WithID(r.Context(), organizationID)))
	})
}

func hasFeatureAllFlags(needles, haystack []string) bool {
	for _, f := range needles {
		found := false
		for _, has := range haystack {
			if f == has {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// AuthAdminMiddleware is a middleware.Interface for authentication probes based on the headers
type AuthAdminMiddleware struct {
	Authenticator Authenticator
	OutputHeader  string
}

// Wrap implements middleware.Interface
func (a AuthAdminMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminID, err := a.Authenticator.AuthenticateAdmin(w, r)
		if err != nil {
			if unauth, ok := err.(*Unauthorized); ok {
				log.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				log.Errorf("proxy: error contacting authenticator: %v", err)
				w.WriteHeader(http.StatusBadGateway)
			}
			return
		}

		r.Header.Add(a.OutputHeader, adminID)
		next.ServeHTTP(w, r.WithContext(user.WithID(r.Context(), adminID)))
	})
}

// AuthUserMiddleware is a middleware.Interface for authentication users based on the
// cookie (and not to any specific org)
type AuthUserMiddleware struct {
	Authenticator       Authenticator
	UserIDHeader        string
	FeatureFlagsHeader  string
	RequireFeatureFlags []string
}

// Wrap implements middleware.Interface
func (a AuthUserMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := a.Authenticator.AuthenticateUser(w, r)
		if err != nil {
			if unauth, ok := err.(*Unauthorized); ok {
				log.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				log.Errorf("proxy: error contacting authenticator: %v", err)
				w.WriteHeader(http.StatusBadGateway)
			}
			return
		}

		r.Header.Add(a.UserIDHeader, userID)
		next.ServeHTTP(w, r)
	})
}
