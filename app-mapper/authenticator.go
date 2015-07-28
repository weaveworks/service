package main

import (
	"net/http"
)

type Authenticator interface {
	Authenticate(r *http.Request) (*AuthenticatorResponse, error)
}

type AuthenticatorResponse struct {
	OrganizationId string
	HttpStatus     int
}

type MockAuthenticator struct{}

func (m *MockAuthenticator) Authenticate(r *http.Request) (*AuthenticatorResponse, error) {
	return &AuthenticatorResponse{"", http.StatusOK}, nil
}

type WebAuthenticator struct {
	ServerHost string
}

func (m *WebAuthenticator) Authenticate(r *http.Request) (*AuthenticatorResponse, error) {
	// TODO
	return &AuthenticatorResponse{"", http.StatusOK}, nil
}
