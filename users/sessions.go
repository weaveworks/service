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
	cookieName      = "_weave_scope_session"
)

var (
	errInvalidAuthenticationData = errors.New("Invalid authentication data")
)

func setupSessions(validationSecret string, storage findUserByIDer) sessionStore {
	secretBytes := []byte(validationSecret)
	if len(secretBytes) != 64 {
		logrus.Fatal("session-secret must be 64 bytes")
	}

	return sessionStore{
		secret:  validationSecret,
		storage: storage,
		encoder: securecookie.New(secretBytes, nil).SetSerializer(securecookie.JSONEncoder{}),
	}
}

type sessionStore struct {
	secret  string
	storage findUserByIDer
	encoder *securecookie.SecureCookie
}

type session struct {
	UserID    string
	CreatedAt time.Time
}

func (s sessionStore) Get(r *http.Request) (*user, error) {
	cookie, err := r.Cookie(cookieName)
	if err == http.ErrNoCookie {
		err = errInvalidAuthenticationData
	}
	if err != nil {
		return nil, err
	}
	return s.Decode(cookie.Value)
}

func (s sessionStore) Decode(encoded string) (*user, error) {
	// Parse and validate the encoded session
	var session session
	if err := s.encoder.Decode(cookieName, encoded, &session); err != nil {
		return nil, errInvalidAuthenticationData
	}
	// Check the session hasn't expired
	if session.CreatedAt.IsZero() || time.Now().UTC().Sub(session.CreatedAt) > sessionDuration {
		return nil, errInvalidAuthenticationData
	}
	// Lookup the user by encoded id
	user, err := s.storage.FindUserByID(session.UserID)
	switch {
	case err == errNotFound:
		return nil, errInvalidAuthenticationData
	case err != nil:
		return nil, err
	case user.ApprovedAt.IsZero():
		return nil, errInvalidAuthenticationData
	}

	return user, nil
}

func (s sessionStore) Set(w http.ResponseWriter, userID string) error {
	cookie, err := s.Cookie(userID)
	if err == nil {
		http.SetCookie(w, cookie)
	}
	return err
}

func (s sessionStore) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func (s sessionStore) Cookie(userID string) (*http.Cookie, error) {
	value, err := s.Encode(userID)
	return &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().UTC().Add(sessionDuration),
	}, err
}

func (s sessionStore) Encode(userID string) (string, error) {
	return s.encoder.Encode(cookieName, session{
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
	})
}
