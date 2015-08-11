package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
)

type authenticator interface {
	authenticate(r *http.Request) (authenticatorResponse, error)
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

func (m *mockAuthenticator) authenticate(r *http.Request) (authenticatorResponse, error) {
	return authenticatorResponse{"mockID"}, nil
}

type webAuthenticator struct {
	serverHost string
}

const (
	authCookieName = "_weave_run_session"
	authHeaderName = "Authorization"
)

func (m *webAuthenticator) authenticate(r *http.Request) (authenticatorResponse, error) {
	// Extract Authorization cookie and/or the Authorization header to inject them in the
	// lookup request. If the cookie and the header were not set, don't even bother to do a
	// lookup.
	authCookie, err := r.Cookie(authCookieName)
	authHeader := r.Header.Get(authHeaderName)
	if err != nil && authHeader == "" {
		logrus.Error("authenticator: tried to authenticate request without credentials")
		return authenticatorResponse{}, &unauthorized{http.StatusUnauthorized}
	}

	lookupReq := m.buildLookupRequest(authCookie, authHeader)

	// Contact the authorization server
	client := &http.Client{}
	res, err := client.Do(lookupReq)
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

func (m *webAuthenticator) buildLookupRequest(authCookie *http.Cookie, authHeader string) *http.Request {
	url := fmt.Sprintf("http://%s/private/api/users/lookup", m.serverHost)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Fatal("authenticator: cannot build lookup request: ", err)
	}

	if len(authHeader) > 0 {
		req.Header.Set(authHeaderName, authHeader)
	}
	if authCookie != nil {
		req.AddCookie(authCookie)
	}
	return req
}
