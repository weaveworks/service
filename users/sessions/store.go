package sessions

import (
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/securecookie"

	"github.com/weaveworks/service/users"
)

const (
	sessionDuration = 1440 * time.Hour

	// CookieName is the name of the cookie we set for the session. It is exposed
	// for testing.
	CookieName = "_weave_scope_session"
)

// MustNewStore creates a new session store, or panics.
func MustNewStore(validationSecret string, finder users.FindUserByIDer) Store {
	secretBytes := []byte(validationSecret)
	if len(secretBytes) != 64 {
		logrus.Fatal("session-secret must be 64 bytes")
	}

	return Store{
		secret:  validationSecret,
		finder:  finder,
		encoder: securecookie.New(secretBytes, nil).SetSerializer(securecookie.JSONEncoder{}),
	}
}

// Store is a session store. It manages reading and writing from cookies.
type Store struct {
	secret  string
	finder  users.FindUserByIDer
	encoder *securecookie.SecureCookie
}

type session struct {
	UserID     string
	ProviderID string
	CreatedAt  time.Time
}

// Get fetches the current session for this request.
func (s Store) Get(r *http.Request) (*users.User, error) {
	cookie, err := r.Cookie(CookieName)
	if err == http.ErrNoCookie {
		err = users.ErrInvalidAuthenticationData
	}
	if err != nil {
		return nil, err
	}
	return s.Decode(cookie.Value)
}

// Decode converts an encoded session in a user.
func (s Store) Decode(encoded string) (*users.User, error) {
	// Parse and validate the encoded session
	var session session
	if err := s.encoder.Decode(CookieName, encoded, &session); err != nil {
		return nil, users.ErrInvalidAuthenticationData
	}
	// Check the session hasn't expired
	if session.CreatedAt.IsZero() || time.Now().UTC().Sub(session.CreatedAt) > sessionDuration {
		return nil, users.ErrInvalidAuthenticationData
	}
	// Lookup the user by encoded id
	user, err := s.finder.FindUserByID(session.UserID)
	switch {
	case err == users.ErrNotFound:
		return nil, users.ErrInvalidAuthenticationData
	case err != nil:
		return nil, err
	case user.ApprovedAt.IsZero():
		return nil, users.ErrInvalidAuthenticationData
	}

	return user, nil
}

// Set stores the session with the given userID, and providerID, for the user.
func (s Store) Set(w http.ResponseWriter, userID, providerID string) error {
	cookie, err := s.Cookie(userID, providerID)
	if err == nil {
		http.SetCookie(w, cookie)
	}
	return err
}

// Clear deletes session data for the response
func (s Store) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// Cookie creates the http cookie to set for this user's session.
func (s Store) Cookie(userID, providerID string) (*http.Cookie, error) {
	value, err := s.Encode(userID, providerID)
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().UTC().Add(sessionDuration),
	}, err
}

// Encode converts the session data into a session string
func (s Store) Encode(userID, providerID string) (string, error) {
	return s.encoder.Encode(CookieName, session{
		UserID:     userID,
		ProviderID: providerID,
		CreatedAt:  time.Now().UTC(),
	})
}
