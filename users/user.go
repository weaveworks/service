package users

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
)

// FindUserByIDer is an interface of just FindUserByID, for loosely coupling
// things to the db.DB
type FindUserByIDer interface {
	FindUserByID(ctx context.Context, id string) (*User, error)
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// FormatCreatedAt formats the user's created at timestamp
func (u *User) FormatCreatedAt() string {
	return formatTimestamp(u.CreatedAt)
}

// FormatFirstLoginAt formats the user's first login timestamp
func (u *User) FormatFirstLoginAt() string {
	return formatTimestamp(u.FirstLoginAt)
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
