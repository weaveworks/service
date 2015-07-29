package main

import (
	"encoding/json"
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
	organizationID string
}

type mockAuthenticator struct{}

func (m *mockAuthenticator) authenticate(r *http.Request) (authenticatorResponse, error) {
	return authenticatorResponse{"mockOrganizationID"}, nil
}

type webAuthenticator struct {
	serverHost string
}

func (m *webAuthenticator) authenticate(r *http.Request) (authenticatorResponse, error) {
	var authRes authenticatorResponse
	// Extract _weave_session cookie and/or the Authorization header to
	// inject them in the lookup request
	sessionCookie, errCookie := r.Cookie("_weave_session")
	authHeader := r.Header.Get("Authorization")

	// If the cookie and the header were not set, don't even bother to do a
	// lookup
	if errCookie != nil && len(authHeader) == 0 {
		return authRes, &unauthorized{http.StatusUnauthorized}
	}

	// Contact the authorization server
	client := &http.Client{}
	lookupReq := m.newLookupRequest()
	if len(authHeader) == 0 {
		lookupReq.Header.Set("Authorization", authHeader)
	}
	if errCookie != nil {
		lookupReq.AddCookie(sessionCookie)
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
	} else {
		return authRes, &unauthorized{res.StatusCode}
	}

	return authRes, nil
}

func (m *webAuthenticator) newLookupRequest() *http.Request {
	url := fmt.Sprintf("http://%s/private/lookup", m.serverHost)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Fatal("authenticator: newLookupRequest() unexpectedly failed")
	}
	return req
}
