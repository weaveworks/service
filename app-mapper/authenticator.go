package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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
	return authenticatorResponse{"mockOrganizationID"}, nil
}

type webAuthenticator struct {
	serverHost string
}

const (
	authLookupPath = "/private/lookup"
	authCookieName = "_weave_run_session"
	authHeaderName = "Authorization"
)

func (m *webAuthenticator) authenticate(r *http.Request) (authenticatorResponse, error) {
	var authRes authenticatorResponse
	// Extract Authorization cookie and/or the Authorization header to inject them in the
	// lookup request
	authCookie, errCookie := r.Cookie(authCookieName)
	authHeader := r.Header.Get(authHeaderName)

	// If the cookie and the header were not set, don't even bother to do a
	// lookup
	if errCookie != nil && len(authHeader) == 0 {
		return authRes, &unauthorized{http.StatusUnauthorized}
	}

	// Contact the authorization server
	client := &http.Client{}
	lookupReq := m.newLookupRequest()
	if len(authHeader) > 0 {
		lookupReq.Header.Set(authHeaderName, authHeader)
	}
	if errCookie == nil {
		lookupReq.AddCookie(authCookie)
	}
	res, err := client.Do(lookupReq)
	if err != nil {
		return authRes, err
	}
	defer res.Body.Close()

	// Parse the response
	if res.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return authRes, err
		}
		err = json.Unmarshal(body, &authRes)
		if err != nil {
			return authRes, err
		}
		if len(authRes.OrganizationID) == 0 {
			return authRes, errors.New("empty OrganizationID")
		}

	} else {
		return authRes, &unauthorized{res.StatusCode}
	}

	return authRes, nil
}

func (m *webAuthenticator) newLookupRequest() *http.Request {
	url := fmt.Sprintf("http://%s"+authLookupPath, m.serverHost)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Fatal("authenticator: newLookupRequest() unexpectedly failed")
	}
	return req
}
