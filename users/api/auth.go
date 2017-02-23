package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

// Credentials are what gets parsed from ParseAuthorizationHeader
type Credentials struct {
	Realm  string
	Params map[string]string
}

// ParseAuthorizationHeader parses an auth header into Credentials, if possible.
func ParseAuthorizationHeader(r *http.Request) (*Credentials, bool) {
	header := r.Header.Get("Authorization")
	for _, realm := range []string{"Basic", "Bearer"} {
		prefix := realm + " "
		if strings.HasPrefix(header, prefix) {
			k := strings.ToLower(realm)
			return &Credentials{
				Realm:  realm,
				Params: map[string]string{k: strings.TrimPrefix(header, prefix)},
			}, true
		}
	}
	i := strings.IndexByte(header, ' ')
	if i == -1 {
		return nil, false
	}

	c := &Credentials{Realm: header[:i], Params: map[string]string{}}
	for _, field := range strings.Split(header[i+1:], ",") {
		if i := strings.IndexByte(field, '='); i == -1 {
			c.Params[field] = ""
		} else {
			c.Params[field[:i]] = field[i+1:]
		}
	}
	return c, true
}

// authenticateUser authenticates a user, passing that directly to the handler
func (a *API) authenticateUser(handler func(*users.User, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return a.authenticateUserVia(
		handler,
		UserAuthenticator(a.cookieAuth),
		UserAuthenticator(a.apiTokenAuth),
	)
}

// authenticateProbe tries authenticating via all known methods
func (a *API) authenticateProbe(handler func(*users.Organization, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return a.authenticateInstanceVia(
		handler,
		OrganizationAuthenticator(a.probeTokenAuth),
	)
}

// UserAuthenticator can authenticate user requests
type UserAuthenticator func(w http.ResponseWriter, r *http.Request) (*users.User, error)

// OrganizationAuthenticator can authenticate organization requests
type OrganizationAuthenticator func(w http.ResponseWriter, r *http.Request) (*users.Organization, error)

func (a *API) authenticateUserVia(handler func(*users.User, http.ResponseWriter, *http.Request), strategies ...UserAuthenticator) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			user *users.User
			err  error
		)
		for _, s := range strategies {
			user, err = s(w, r)
			if err != nil {
				continue
			}
			// User actions always go through this endpoint because authfe checks the
			// authentication endpoint every time. We use this to tell marketing about
			// login activity.
			a.marketingQueues.UserAccess(user.Email, time.Now())
			handler(user, w, r)
			return
		}

		// convert not found errors, which we expect, into invalid auth
		if err == users.ErrNotFound {
			err = users.ErrInvalidAuthenticationData
		}
		render.Error(w, r, err)
		return
	})
}

func (a *API) authenticateInstanceVia(handler func(*users.Organization, http.ResponseWriter, *http.Request), strategies ...OrganizationAuthenticator) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			auth *users.Organization
			err  error
		)
		for _, s := range strategies {
			auth, err = s(w, r)
			if err != nil {
				continue
			}
			handler(auth, w, r)
			return
		}

		// convert not found errors, which we expect, into invalid auth
		if err == users.ErrNotFound {
			err = users.ErrInvalidAuthenticationData
		}
		render.Error(w, r, err)
		return
	})
}

func (a *API) cookieAuth(w http.ResponseWriter, r *http.Request) (*users.User, error) {
	// try logging in by cookie
	userID, err := a.sessions.Get(r)
	if err != nil {
		return nil, err
	}
	u, err := a.db.FindUserByID(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	// Update the cookie expiry:
	if err := a.sessions.Set(w, userID); err != nil {
		return nil, err
	}
	return u, nil
}

func (a *API) apiTokenAuth(w http.ResponseWriter, r *http.Request) (*users.User, error) {
	// try logging in by user token header
	credentials, ok := ParseAuthorizationHeader(r)
	if !ok || credentials.Realm != "Scope-User" {
		return nil, users.ErrInvalidAuthenticationData
	}

	token, ok := credentials.Params["token"]
	if !ok {
		return nil, users.ErrInvalidAuthenticationData
	}

	u, err := a.db.FindUserByAPIToken(r.Context(), token)
	if err != nil {
		return nil, err
	}

	return u, nil
}

func (a *API) probeTokenAuth(w http.ResponseWriter, r *http.Request) (*users.Organization, error) {
	// try logging in by probe token header
	credentials, ok := ParseAuthorizationHeader(r)
	if !ok || credentials.Realm != "Scope-Probe" {
		return nil, users.ErrInvalidAuthenticationData
	}

	token, ok := credentials.Params["token"]
	if !ok {
		return nil, users.ErrInvalidAuthenticationData
	}

	o, err := a.db.FindOrganizationByProbeToken(r.Context(), token)
	if err != nil {
		return nil, err
	}

	return o, nil
}
