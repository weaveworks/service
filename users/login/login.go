package login

import (
	"encoding/json"
	"time"
)

// Claims made by the OIDC provider, e.g. auth0
type Claims struct {
	ID           string       `json:"sub,omitempty"`
	Nickname     string       `json:"nickname,omitempty"`
	Email        string       `json:"email,omitempty"`
	Verified     bool         `json:"email_verified,omitempty"`
	Name         string       `json:"name,omitempty"`
	GivenName    string       `json:"given_name,omitempty"`
	FamilyName   string       `json:"family_name,omitempty"`
	UserMetadata UserMetadata `json:"user_metadata,omitempty"`
}

// UserMetadata contains custom claims we can always write to
type UserMetadata struct {
	CompanyName string `json:"company_name,omitempty"`
}

// Login pairs a user with a login provider they've used (can use) to log in.
type Login struct {
	UserID     string
	Provider   string
	ProviderID string
	Session    json.RawMessage // per-user session information for configuring the client
	CreatedAt  time.Time
}
