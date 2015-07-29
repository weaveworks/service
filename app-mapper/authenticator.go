package main

import (
	"net/http"
)

type authenticator interface {
	authenticate(r *http.Request) (authenticatorResponse, error)
}

type authenticatorResponse struct {
	organizationID string
	httpStatus     int
}

type mockAuthenticator struct{}

func (m *mockAuthenticator) authenticate(r *http.Request) (authenticatorResponse, error) {
	return authenticatorResponse{"", http.StatusOK}, nil
}

type webAuthenticator struct {
	ServerHost string
}

func (m *webAuthenticator) authenticate(r *http.Request) (authenticatorResponse, error) {
	// TODO
	return authenticatorResponse{"", http.StatusOK}, nil
}
