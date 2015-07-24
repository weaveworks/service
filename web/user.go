package main

import (
	"fmt"
	htmlTemplate "html/template"
	"net/url"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID               string
	Email            string
	OrganizationID   string
	OrganizationName string
	Token            string
	TokenExpiry      time.Time
	ApprovedAt       time.Time
	FirstLoginAt     time.Time
	LastLoginAt      time.Time
}

func (u *User) CompareToken(other string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(u.Token), []byte(other)); err != nil {
		return false
	}
	return time.Now().UTC().Before(u.TokenExpiry)
}

func (u *User) LoginURL(rawToken string) string {
	params := url.Values{}
	params.Set("email", u.Email)
	params.Set("token", rawToken)
	return fmt.Sprintf("http://%s/users/signup?%s", domain, params.Encode())
}

func (u *User) LoginLink(rawToken string) htmlTemplate.HTML {
	url := u.LoginURL(rawToken)
	return htmlTemplate.HTML(
		fmt.Sprintf(
			"<a href=\"%s\">%s</a>",
			url,
			htmlTemplate.HTMLEscapeString(url),
		),
	)
}
