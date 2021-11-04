package login

import (
	"context"
	"encoding/json"
	"net/http"
)

// Provider is an interface for a login provider
// It's always auth0 in practice, but is an interface so it can be mocked
type Provider interface {
	// Called after the provider has authenticated the user, to verify
	// the response and decode the data
	Login(r *http.Request) (*Claims, json.RawMessage, map[string]string, error)
	// Get URL for a specific connection
	LoginURL(r *http.Request, connection string) string
	// Get URL to logout with the provider
	LogoutURL(r *http.Request) string

	// Create an invite - this creates a new user and initializes a passwordless
	// login for them
	InviteUser(email string, inviter string, teamName string) error

	// Update claims for an existing user.
	UpdateClaims(ctx context.Context, claims Claims, session json.RawMessage) error

	// Initiate a passwordless login, e.g. send an email
	// If the request isn't provided, the email won't have valid state, which will fail Login
	PasswordlessLogin(r *http.Request, email string) error

	// Retrieves the access token for the actual provider (e.g. for github/google,
	// not auth0) given a auth0 user ID. Expects the caller to check authorization
	GetAccessToken(string) (*string, error)
}
