package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/securecookie"
)

const (
	sessionDuration = 1440 * time.Hour
	cookieName      = "_weave_session"
)

var (
	ErrInvalidAuthenticationData = errors.New("invalid authentication data")
)

func setupSessions(validationSecret string) {
	secretBytes := []byte(validationSecret)
	if len(secretBytes) != 64 {
		logrus.Fatal("session-secret must be 64 bytes")
	}

	sessions = SessionStore{
		secret:  validationSecret,
		encoder: securecookie.New(secretBytes, nil).SetSerializer(securecookie.JSONEncoder{}),
	}
}

type SessionStore struct {
	secret  string
	encoder *securecookie.SecureCookie
}

type Session struct {
	UserID    string
	CreatedAt time.Time
}

func (s SessionStore) Get(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, err
	}
	return s.Decode(cookie.Value)
}

func (s SessionStore) Decode(encoded string) (*User, error) {
	// Parse and validate the encoded session
	var session Session
	if err := s.encoder.Decode(cookieName, encoded, &session); err != nil {
		logrus.Debugf("Error decoding session: %s", err)
		return nil, ErrInvalidAuthenticationData
	}
	// Check the session hasn't expired
	if session.CreatedAt.IsZero() || time.Now().UTC().Sub(session.CreatedAt) > sessionDuration {
		return nil, ErrInvalidAuthenticationData
	}
	// Lookup the user by encoded id
	user, err := storage.FindUserByID(session.UserID)
	if err == ErrNotFound {
		err = ErrInvalidAuthenticationData
	}
	return user, err
}

func (s SessionStore) Set(w http.ResponseWriter, userID string) error {
	value, err := s.Encode(userID)
	if err == nil {
		http.SetCookie(w, &http.Cookie{
			Name:    cookieName,
			Domain:  domain,
			Value:   value,
			Path:    "/",
			Expires: time.Now().UTC().Add(sessionDuration),
		})
	}
	return err
}

func (s SessionStore) Encode(userID string) (string, error) {
	return s.encoder.Encode(cookieName, Session{
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
	})
}
