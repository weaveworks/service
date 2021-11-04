package login

import (
	"github.com/coreos/go-oidc"
)

// NewGoogleConnection returns a Connection that describes google logins
// This does not handle google workspace extra data
func NewGoogleConnection() Connection {
	return Connection{
		Scopes:         []string{oidc.ScopeOpenID, "profile", "email"},
		ConnectionName: "google-oauth2",
	}
}
