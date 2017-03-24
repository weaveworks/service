package sessions

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/securecookie"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/client"
)

const (
	// SessionDuration is the duration used to set expiration session cookies
	SessionDuration = 1440 * time.Hour
)

// MustNewStore creates a new session store, or panics.
func MustNewStore(validationSecret string, secure bool) Store {
	secretBytes := []byte(validationSecret)
	if len(secretBytes) != 64 {
		logrus.Fatal("session-secret must be 64 bytes")
	}

	return Store{
		secret:  validationSecret,
		encoder: securecookie.New(secretBytes, nil).SetSerializer(securecookie.JSONEncoder{}),
		secure:  secure,
	}
}

// Store is a session store. It manages reading and writing from cookies.
type Store struct {
	secret  string
	encoder *securecookie.SecureCookie
	secure  bool
}

// Session is the decoded representation of a session cookie
type Session struct {
	UserID    string
	CreatedAt time.Time
}

// Get fetches the current session for this request.
func (s Store) Get(r *http.Request) (Session, error) {
	value, err := Extract(r)
	if err != nil {
		return Session{}, err
	}
	return s.Decode(value)
}

// Extract the encoded session from a request.
func Extract(r *http.Request) (string, error) {
	cookie, err := r.Cookie(client.AuthCookieName)
	if err == http.ErrNoCookie {
		err = users.NewInvalidAuthenticationDataError(err)
	}
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// Decode converts an encoded session into a user ID.
func (s Store) Decode(encoded string) (Session, error) {
	// Parse and validate the encoded session
	var session Session
	if err := s.encoder.Decode(client.AuthCookieName, encoded, &session); err != nil {
		return Session{}, users.NewInvalidAuthenticationDataError(err)
	}
	// Check the session hasn't expired
	if session.CreatedAt.IsZero() || time.Now().UTC().Sub(session.CreatedAt) > SessionDuration {
		return Session{}, users.NewInvalidAuthenticationDataError(fmt.Errorf("session for userID %v expired", session.UserID))
	}
	// Lookup the user by encoded id
	if session.UserID == "" {
		return Session{}, users.NewInvalidAuthenticationDataError(fmt.Errorf("empty session userID"))
	}
	return session, nil
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
		Secure:   s.secure,
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
		Expires:  time.Now().UTC().Add(SessionDuration),
		MaxAge:   int(SessionDuration / time.Second),
		Secure:   s.secure,
	}, err
}

// Encode converts the session data into a session string
func (s Store) Encode(userID string) (string, error) {
	return s.encoder.Encode(client.AuthCookieName, Session{
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
	})
}
