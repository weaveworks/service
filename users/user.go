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
