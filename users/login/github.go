package login

import (
	"github.com/coreos/go-oidc"
)

// NewGithubConnection returns a Connection that describes github logins
func NewGithubConnection() Connection {
	return Connection{
		Scopes:         []string{oidc.ScopeOpenID, "profile", "email", "repo", "write:public_key", "read:public_key"},
		ConnectionName: "github",
	}
}
