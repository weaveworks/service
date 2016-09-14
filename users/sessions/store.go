package sessions

import (
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/securecookie"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/client"
)

const (
	sessionDuration = 1440 * time.Hour
)

// MustNewStore creates a new session store, or panics.
func MustNewStore(validationSecret string) Store {
	secretBytes := []byte(validationSecret)
	if len(secretBytes) != 64 {
		logrus.Fatal("session-secret must be 64 bytes")
	}

	return Store{
		secret:  validationSecret,
		encoder: securecookie.New(secretBytes, nil).SetSerializer(securecookie.JSONEncoder{}),
	}
}

// Store is a session store. It manages reading and writing from cookies.
type Store struct {
	secret  string
	encoder *securecookie.SecureCookie
}

type session struct {
	UserID    string
	CreatedAt time.Time
}

// Get fetches the current session for this request.
func (s Store) Get(r *http.Request) (string, error) {
	cookie, err := r.Cookie(client.AuthCookieName)
	if err == http.ErrNoCookie {
		err = users.ErrInvalidAuthenticationData
	}
	if err != nil {
		return "", err
	}
	return s.Decode(cookie.Value)
}

// Decode converts an encoded session into a user ID.
func (s Store) Decode(encoded string) (string, error) {
	// Parse and validate the encoded session
	var session session
	if err := s.encoder.Decode(client.AuthCookieName, encoded, &session); err != nil {
		return "", users.ErrInvalidAuthenticationData
	}
	// Check the session hasn't expired
	if session.CreatedAt.IsZero() || time.Now().UTC().Sub(session.CreatedAt) > sessionDuration {
		return "", users.ErrInvalidAuthenticationData
	}
	// Lookup the user by encoded id
	if session.UserID == "" {
		return "", users.ErrInvalidAuthenticationData
	}
	return session.UserID, nil
}

// Set stores the session with the given userID for the user.
func (s Store) Set(w http.ResponseWriter, userID string) error {
	cookie, err := s.Cookie(userID)
	if err == nil {
		http.SetCookie(w, cookie)
	}
	return err
}

// Clear deletes session data for the response
func (s Store) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     client.AuthCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().UTC().Add(-1 * time.Second),
		MaxAge:   -1,
	})
}

// Cookie creates the http cookie to set for this user's session.
func (s Store) Cookie(userID string) (*http.Cookie, error) {
	value, err := s.Encode(userID)
	return &http.Cookie{
		Name:     client.AuthCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().UTC().Add(sessionDuration),
		MaxAge:   int(sessionDuration / time.Second),
	}, err
}

// Encode converts the session data into a session string
func (s Store) Encode(userID string) (string, error) {
	return s.encoder.Encode(client.AuthCookieName, session{
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
	})
}
