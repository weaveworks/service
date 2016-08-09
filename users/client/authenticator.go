package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/bluele/gcache"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	authCacheCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "scope",
		Name:      "auth_cache",
		Help:      "Reports fetches that miss local cache.",
	}, []string{"cache", "result"})
)

func init() {
	prometheus.MustRegister(authCacheCounter)
}

// Authenticator is the client interface to the users service.
type Authenticator interface {
	AuthenticateOrg(r *http.Request, orgExternalID string) (string, error)
	AuthenticateProbe(r *http.Request) (string, error)
	AuthenticateAdmin(r *http.Request) (string, error)
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

func (mockAuthenticator) AuthenticateOrg(r *http.Request, orgExternalID string) (string, error) {
	return "mockID", nil
}

func (mockAuthenticator) AuthenticateProbe(r *http.Request) (string, error) {
	return "mockID", nil
}

func (mockAuthenticator) AuthenticateAdmin(r *http.Request) (string, error) {
	return "mockID", nil
}

type webAuthenticator struct {
	url            string
	client         http.Client
	probeCredCache gcache.Cache
	orgCredCache   gcache.Cache
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
	cookie, orgExternalID string
}

func (m *webAuthenticator) AuthenticateOrg(r *http.Request, orgExternalID string) (string, error) {
	// Extract Authorization cookie to inject it in the lookup request. If it were
	// not set, don't even bother to do a lookup.
	authCookie, err := r.Cookie(AuthCookieName)
	if err != nil {
		log.Error("authenticator: org: tried to authenticate request without credentials")
		return "", &Unauthorized{http.StatusUnauthorized}
	}

	cacheKey := orgCredCacheKey{authCookie.Value, orgExternalID}
	if m.orgCredCache != nil {
		org, err := m.orgCredCache.Get(cacheKey)
		authCacheCounter.WithLabelValues("org_cred_cache", hitOrMiss(err)).Inc()
		if err == nil {
			return org.(string), nil
		}
	}

	url := fmt.Sprintf("%s/private/api/users/lookup/%s", m.url, url.QueryEscape(orgExternalID))
	lookupReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("authenticator: cannot build lookup request: ", err)
		return "", err
	}
	lookupReq.AddCookie(authCookie)
	org, err := m.decodeOrg(m.doAuthenticateRequest(lookupReq))
	if err == nil && m.orgCredCache != nil {
		m.orgCredCache.Set(cacheKey, org)
	}
	return org, err
}

func getAuthHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get(AuthHeaderName)
	if strings.HasPrefix(authHeader, "Scope-Probe") {
		return authHeader, nil
	}

	// To allow grafana to talk to the service, we also accept basic auth,
	// ignoring the username and treating the password as the token.
	_, token, ok := r.BasicAuth()
	if ok {
		return fmt.Sprintf("Scope-Probe token=%s", token), nil
	}

	log.Error("authenticator: probe: tried to authenticate request without credentials")
	return "", &Unauthorized{http.StatusUnauthorized}
}

func (m *webAuthenticator) AuthenticateProbe(r *http.Request) (string, error) {
	// Extract Authorization header to inject it in the lookup request. If
	// it were not set, don't even bother to do a lookup.
	authHeader, err := getAuthHeader(r)
	if err != nil {
		return "", err
	}

	if m.probeCredCache != nil {
		org, err := m.probeCredCache.Get(authHeader)
		authCacheCounter.WithLabelValues("probe_cred_cache", hitOrMiss(err)).Inc()
		if err == nil {
			return org.(string), nil
		}
	}

	url := fmt.Sprintf("%s/private/api/users/lookup", m.url)
	lookupReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("authenticator: cannot build lookup request: ", err)
		return "", err
	}
	lookupReq.Header.Set(AuthHeaderName, authHeader)
	org, err := m.decodeOrg(m.doAuthenticateRequest(lookupReq))
	if err == nil && m.probeCredCache != nil {
		m.probeCredCache.Set(authHeader, org)
	}
	return org, err
}

func (m *webAuthenticator) AuthenticateAdmin(r *http.Request) (string, error) {
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
	return m.decodeAdmin(m.doAuthenticateRequest(lookupReq))
}

func (m *webAuthenticator) doAuthenticateRequest(r *http.Request) (io.ReadCloser, error) {
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
	return res.Body, nil
}

func (m *webAuthenticator) decodeOrg(body io.ReadCloser, err error) (string, error) {
	if err != nil {
		return "", err
	}
	defer body.Close()
	var authRes struct {
		OrganizationID string `json:"organizationID"`
	}
	if err := json.NewDecoder(body).Decode(&authRes); err != nil {
		return "", err
	}
	if authRes.OrganizationID == "" {
		return "", errors.New("empty OrganizationID")
	}
	return authRes.OrganizationID, nil
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

// AuthOrgMiddleware is a middleware.Interface for authentication organisations based on the
// cookie and an org name in the path
type AuthOrgMiddleware struct {
	Authenticator Authenticator
	OrgExternalID func(*http.Request) (string, bool)
	OutputHeader  string
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

		organizationID, err := a.Authenticator.AuthenticateOrg(r, orgExternalID)
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

		r.Header.Add(a.OutputHeader, organizationID)
		next.ServeHTTP(w, r)
	})
}

// AuthProbeMiddleware is a middleware.Interface for authentication probes based on the headers
type AuthProbeMiddleware struct {
	Authenticator Authenticator
	OutputHeader  string
}

// Wrap implements middleware.Interface
func (a AuthProbeMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := a.Authenticator.AuthenticateProbe(r)
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

		r.Header.Add(a.OutputHeader, organizationID)
		next.ServeHTTP(w, r)
	})
}

// AuthAdminMiddleware is a middleware.Interface for authentication probes based on the headers
type AuthAdminMiddleware struct {
	Authenticator Authenticator
	OutputHeader  string
}

// Wrap implements middleware.Interface
func (a AuthAdminMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminID, err := a.Authenticator.AuthenticateAdmin(r)
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
		next.ServeHTTP(w, r)
	})
}
