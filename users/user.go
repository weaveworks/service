package main

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type user struct {
	ID             string
	Email          string
	Token          string
	TokenCreatedAt time.Time
	ApprovedAt     time.Time
	FirstLoginAt   time.Time
	CreatedAt      time.Time
	Organization   *organization
}

func (u *user) CompareToken(other string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(u.Token), []byte(other)); err != nil {
		return false
	}
	return time.Now().UTC().Sub(u.TokenCreatedAt) <= 6*time.Hour
}

func (u *user) IsApproved() bool {
	return !u.ApprovedAt.IsZero()
}

type usersApprovedFilter bool

func newUsersApprovedFilter(s []string) filter {
	return usersApprovedFilter(len(s) > 0 && s[0] == "true")
}

type usersOrganizationFilter string

func newUsersOrganizationFilter(s []string) filter {
	if len(s) == 0 {
		return nil
	}
	return usersOrganizationFilter(s[0])
}
