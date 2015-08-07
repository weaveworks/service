package main

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID               string
	Email            string
	OrganizationID   string
	OrganizationName string
	Token            string
	TokenCreatedAt   time.Time
	ApprovedAt       time.Time
	FirstLoginAt     time.Time
	LastLoginAt      time.Time
	CreatedAt        time.Time
}

func (u *User) GenerateToken() (string, error) {
	return secureRandomBase64(20)
}

func secureRandomBase64(charCount int) (string, error) {
	byteCount := (charCount * 3) / 4
	randomData := make([]byte, byteCount)
	_, err := rand.Read(randomData)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(randomData), nil
}

func (u *User) CompareToken(other string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(u.Token), []byte(other)); err != nil {
		return false
	}
	return time.Now().UTC().Sub(u.TokenCreatedAt) <= 6*time.Hour
}
