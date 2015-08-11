package main

import (
	"crypto/rand"
	"encoding/base64"
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

func (u *user) GenerateToken() (string, error) {
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

func (u *user) CompareToken(other string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(u.Token), []byte(other)); err != nil {
		return false
	}
	return time.Now().UTC().Sub(u.TokenCreatedAt) <= 6*time.Hour
}
