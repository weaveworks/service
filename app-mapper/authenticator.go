package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Sirupsen/logrus"
)

type authenticator interface {
	authenticateOrg(r *http.Request, orgName string) (authenticatorResponse, error)
	authenticateProbe(r *http.Request) (authenticatorResponse, error)
}

// unauthorized error
type unauthorized struct {
	httpStatus int
}

func (u unauthorized) Error() string {
	return http.StatusText(u.httpStatus)
}

type authenticatorResponse struct {
	OrganizationID string
}

type mockAuthenticator struct{}

func (m *mockAuthenticator) authenticateOrg(r *http.Request, orgName string) (authenticatorResponse, error) {
	return authenticatorResponse{"mockID"}, nil
}

func (m *mockAuthenticator) authenticateProbe(r *http.Request) (authenticatorResponse, error) {
	return authenticatorResponse{"mockID"}, nil
}

type webAuthenticator struct {
	serverHost string
}

const (
	authCookieName = "_weave_run_session"
	authHeaderName = "Authorization"
)

func (m *webAuthenticator) authenticateOrg(r *http.Request, orgName string) (resp authenticatorResponse, err error) {
	// Extract Authorization cookie to inject it in the lookup request. If it were
	// not set, don't even bother to do a lookup.
	authCookie, err := r.Cookie(authCookieName)
	if err != nil {
		logrus.Error("authenticator: org: tried to authenticate request without credentials")
		return authenticatorResponse{}, &unauthorized{http.StatusUnauthorized}
	}

	url := fmt.Sprintf("http://%s/private/api/users/lookup/%s", m.serverHost, url.QueryEscape(orgName))
	lookupReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Fatal("authenticator: cannot build lookup request: ", err)
	}
	lookupReq.AddCookie(authCookie)

	return doAuthenticateRequest(lookupReq)
}

func (m *webAuthenticator) authenticateProbe(r *http.Request) (resp authenticatorResponse, err error) {
	// Extract Authorization header to inject it in the lookup request. If
	// it were not set, don't even bother to do a lookup.
	authHeader := r.Header.Get(authHeaderName)
	if authHeader == "" {
		logrus.Error("authenticator: probe: tried to authenticate request without credentials")
		return authenticatorResponse{}, &unauthorized{http.StatusUnauthorized}
	}

	url := fmt.Sprintf("http://%s/private/api/users/lookup", m.serverHost)
	lookupReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Fatal("authenticator: cannot build lookup request: ", err)
	}
	lookupReq.Header.Set(authHeaderName, authHeader)

	return doAuthenticateRequest(lookupReq)
}

func doAuthenticateRequest(r *http.Request) (authenticatorResponse, error) {
	// Contact the authorization server
	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return authenticatorResponse{}, err
	}
	defer res.Body.Close()

	// Parse the response
	if res.StatusCode != http.StatusOK {
		return authenticatorResponse{}, &unauthorized{res.StatusCode}
	}
	var authRes authenticatorResponse
	if err := json.NewDecoder(res.Body).Decode(&authRes); err != nil {
		return authenticatorResponse{}, err
	}
	if authRes.OrganizationID == "" {
		return authenticatorResponse{}, errors.New("empty OrganizationID")
	}
	return authRes, nil

}

func authOrgHandler(a authenticator, getOrgName func(*http.Request) string, next func(w http.ResponseWriter, r *http.Request, orgID string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authResponse, err := a.authenticateOrg(r, getOrgName(r))
		if err != nil {
			handleAuthError(w, err)
			return
		}
		next(w, r, authResponse.OrganizationID)
	})
}

func authProbeHandler(a authenticator, next func(w http.ResponseWriter, r *http.Request, orgID string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authResponse, err := a.authenticateProbe(r)
		if err != nil {
			handleAuthError(w, err)
			return
		}
		next(w, r, authResponse.OrganizationID)
	})
}

func handleAuthError(w http.ResponseWriter, err error) {
	if unauth, ok := err.(unauthorized); ok {
		logrus.Infof("proxy: unauthorized request: %d", unauth.httpStatus)
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		logrus.Errorf("proxy: error contacting authenticator: %v", err)
		w.WriteHeader(http.StatusBadGateway)
	}
}
