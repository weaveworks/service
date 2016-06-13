package main

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type user struct {
	ID             string        `json:"-"`
	Email          string        `json:"email"`
	Token          string        `json:"-"`
	TokenCreatedAt time.Time     `json:"-"`
	ApprovedAt     time.Time     `json:"-"`
	FirstLoginAt   time.Time     `json:"-"`
	CreatedAt      time.Time     `json:"-"`
	Admin          bool          `json:"-"`
	Organization   *organization `json:"-"`
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.Stamp)
}

func (u *user) FormatCreatedAt() string {
	return formatTimestamp(u.CreatedAt)
}

func (u *user) FormatApprovedAt() string {
	return formatTimestamp(u.ApprovedAt)
}

func (u *user) FormatFirstLoginAt() string {
	return formatTimestamp(u.FirstLoginAt)
}

func (u *user) IsApproved() bool {
	return !u.ApprovedAt.IsZero()
}

func (u *user) CompareToken(other string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(u.Token), []byte(other)); err != nil {
		return false
	}
	return time.Now().UTC().Sub(u.TokenCreatedAt) <= 72*time.Hour
}

func newUsersOrganizationFilter(s []string) filter {
	return inFilter{
		SQLField: "organizations.name",
		Value:    s,
		Allowed: func(i interface{}) bool {
			if u, ok := i.(*user); ok && u.Organization != nil {
				for _, name := range s {
					if u.Organization.Name == name {
						return true
					}
				}
			}
			return false
		},
	}
}
