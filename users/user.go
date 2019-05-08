package users

// This file amends the generated struct by protobuf in users.pb.go

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// AdminRoleID stands for the ID of team admin role
	AdminRoleID = "admin"
	// EditorRoleID stands for the ID of team editor role
	EditorRoleID = "editor"
	// ViewerRoleID stands for the ID of team viewer role
	ViewerRoleID = "viewer"
)

// TrialDuration is how long a user has a free trial
// period before we start charging for it.
const TrialDuration = 14 * 24 * time.Hour

// DefaultRoleID is the role given to a new team member if no role is specified
// used in entry points to the system like API endpoints.
const DefaultRoleID = ViewerRoleID

// FindUserByIDer is an interface of just FindUserByID, for loosely coupling
// things to the db.DB
type FindUserByIDer interface {
	FindUserByID(ctx context.Context, id string) (*User, error)
}

// UserUpdate represents an update for a user
type UserUpdate struct {
	Name      string `json:"name"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Company   string `json:"company"`
}

// UserResponse is the response representation of a users.User
type UserResponse struct {
	Email     string `json:"email"`
	Name      string `json:"name"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Company   string `json:"company"`
}

// UserWithRole is a little struct for passing around a User's role in a org.
type UserWithRole struct {
	User User
	Role Role
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

// FormatLastLoginAt formats the user's first login timestamp
func (u *User) FormatLastLoginAt() string {
	return formatTimestamp(u.LastLoginAt)
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

// TrialExpiresAt returns the timestamp when the trial expires for the user
func (u *User) TrialExpiresAt() time.Time {
	return u.CreatedAt.Add(TrialDuration)
}
