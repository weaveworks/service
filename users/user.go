package users

import (
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/login"
)

// FindUserByIDer is an interface of just FindUserByID, for loosely coupling
// things to the storage.Database
type FindUserByIDer interface {
	FindUserByID(id string) (*User, error)
}

// User is what it's all about.
type User struct {
	ID             string          `json:"-"`
	Email          string          `json:"email"`
	Token          string          `json:"-"`
	TokenCreatedAt time.Time       `json:"-"`
	ApprovedAt     time.Time       `json:"-"`
	FirstLoginAt   time.Time       `json:"-"`
	CreatedAt      time.Time       `json:"-"`
	Admin          bool            `json:"-"`
	Logins         []*login.Login  `json:"-"`
	Organizations  []*Organization `json:"-"`
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.Stamp)
}

// FormatCreatedAt formats the user's created at timestamp
func (u *User) FormatCreatedAt() string {
	return formatTimestamp(u.CreatedAt)
}

// FormatApprovedAt formats the user's approved at timestamp
func (u *User) FormatApprovedAt() string {
	return formatTimestamp(u.ApprovedAt)
}

// FormatFirstLoginAt formats the user's first login timestamp
func (u *User) FormatFirstLoginAt() string {
	return formatTimestamp(u.FirstLoginAt)
}

// IsApproved checks if a user has been approved
func (u *User) IsApproved() bool {
	return !u.ApprovedAt.IsZero()
}

// CompareToken does a cryptographically-secure comparison of the user's login token
func (u *User) CompareToken(other string) bool {
	var (
		token          string
		tokenCreatedAt time.Time
	)
	if u != nil {
		token = u.Token
		tokenCreatedAt = u.TokenCreatedAt
	}
	if err := bcrypt.CompareHashAndPassword([]byte(token), []byte(other)); err != nil {
		return false
	}
	return time.Now().UTC().Sub(tokenCreatedAt) <= 72*time.Hour
}

// HasOrganization checks if a user has access to an organization. For now the
// logic is quite simple.
func (u *User) HasOrganization(externalID string) bool {
	for _, o := range u.Organizations {
		if strings.ToLower(o.ExternalID) == strings.ToLower(externalID) {
			return true
		}
	}
	return false
}
