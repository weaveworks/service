package main

import (
	"time"

	"github.com/Masterminds/squirrel"
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

type usersApprovedFilter bool

func newUsersApprovedFilter(s []string) filter {
	return usersApprovedFilter(len(s) > 0 && s[0] == "true")
}

func (f usersApprovedFilter) Item(i interface{}) bool {
	u, ok := i.(*user)
	if !ok {
		return false
	}
	if bool(f) {
		return u.IsApproved()
	}
	return !u.IsApproved()
}

func (f usersApprovedFilter) Select(q squirrel.SelectBuilder) squirrel.SelectBuilder {
	if bool(f) {
		return q.Where("users.approved_at is not null")
	}
	return q.Where("users.approved_at is null")
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
