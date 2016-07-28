package main

import (
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/users/login"
)

type user struct {
	ID             string          `json:"-"`
	Email          string          `json:"email"`
	Token          string          `json:"-"`
	TokenCreatedAt time.Time       `json:"-"`
	ApprovedAt     time.Time       `json:"-"`
	FirstLoginAt   time.Time       `json:"-"`
	CreatedAt      time.Time       `json:"-"`
	Admin          bool            `json:"-"`
	Logins         []*login.Login  `json:"-"`
	Organizations  []*organization `json:"-"`
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

func (u *user) HasOrganization(externalID string) bool {
	for _, o := range u.Organizations {
		if strings.ToLower(o.ExternalID) == strings.ToLower(externalID) {
			return true
		}
	}
	return false
}

func newUsersOrganizationFilter(s []string) filter {
	return and{
		inFilter{
			SQLField: "organizations.external_id",
			SQLJoins: []string{
				"memberships on (memberships.user_id = users.id)",
				"organizations on (memberships.organization_id = organizations.id)",
			},
			Value: s,
			Allowed: func(i interface{}) bool {
				if u, ok := i.(*user); ok {
					for _, org := range u.Organizations {
						for _, externalID := range s {
							if org.ExternalID == externalID {
								return true
							}
						}
					}
				}
				return false
			},
		},
		inFilter{
			SQLField: "memberships.deleted_at",
			Value:    nil,
			Allowed:  func(i interface{}) bool { return true },
		},
	}
}
