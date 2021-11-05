package login

import (
	"github.com/coreos/go-oidc"
)

// NewEmailConnection returns a Connection that describes the email connection
// Note: email logins happen through PasswordlessLogin, not this interface
func NewEmailConnection() Connection {
	return Connection{
		Scopes:         []string{oidc.ScopeOpenID, "profile", "email"},
		ConnectionName: "email",
	}
}
