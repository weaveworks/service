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

// Authentication is a set of the credentials for the current request. user
// might be nil (if it was a probe token) organizations might be empty (if the
// user has no organizations)
type Authentication struct {
	AuthType      string // Descriptor of the authentication method which succeeded.
	User          *users.User
	Organizations []*users.Organization
}

// authenticateUser authenticates a user, passing that directly to the handler
func (a *API) authenticateUser(handler func(*users.User, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return a.authenticateVia(
		func(auth Authentication, w http.ResponseWriter, r *http.Request) { handler(auth.User, w, r) },
		Authenticator(a.cookieAuth),
		Authenticator(a.apiTokenAuth),
	)
}

// authenticateProbe tries authenticating via all known methods
func (a *API) authenticateProbe(handler func(*users.Organization, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return a.authenticateVia(
		func(auth Authentication, w http.ResponseWriter, r *http.Request) {
			handler(auth.Organizations[0], w, r)
		},
		Authenticator(a.probeTokenAuth),
	)
}

// Authenticator implements Authenticator for functions
type Authenticator func(w http.ResponseWriter, r *http.Request) (Authentication, error)

func (a *API) authenticateVia(handler func(Authentication, http.ResponseWriter, *http.Request), strategies ...Authenticator) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			auth Authentication
			err  error
		)
		for _, s := range strategies {
			auth, err = s(w, r)
			if err != nil {
				continue
			}
			if auth.User != nil {
				// User actions always go through this endpoint because authfe checks the
				// authentication endpoint every time. We use this to tell marketing about
				// login activity.
				a.marketingQueues.UserAccess(auth.User.Email, time.Now())
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

func (a *API) cookieAuth(w http.ResponseWriter, r *http.Request) (Authentication, error) {
	// try logging in by cookie
	userID, err := a.sessions.Get(r)
	if err != nil {
		return Authentication{}, err
	}
	u, err := a.db.FindUserByID(r.Context(), userID)
	if err != nil {
		return Authentication{}, err
	}
	organizations, err := a.db.ListOrganizationsForUserIDs(r.Context(), u.ID)
	if err != nil {
		return Authentication{}, err
	}
	// Update the cookie expiry:
	if err := a.sessions.Set(w, userID); err != nil {
		return Authentication{}, err
	}
	return Authentication{
		AuthType:      "cookie",
		User:          u,
		Organizations: organizations,
	}, nil
}

func (a *API) apiTokenAuth(w http.ResponseWriter, r *http.Request) (Authentication, error) {
	// try logging in by user token header
	credentials, ok := ParseAuthorizationHeader(r)
	if !ok || credentials.Realm != "Scope-User" {
		return Authentication{}, users.ErrInvalidAuthenticationData
	}

	token, ok := credentials.Params["token"]
	if !ok {
		return Authentication{}, users.ErrInvalidAuthenticationData
	}

	u, err := a.db.FindUserByAPIToken(r.Context(), token)
	if err != nil {
		return Authentication{}, err
	}

	organizations, err := a.db.ListOrganizationsForUserIDs(r.Context(), u.ID)
	if err != nil {
		return Authentication{}, err
	}
	return Authentication{
		AuthType:      "api_token",
		User:          u,
		Organizations: organizations,
	}, nil
}

func (a *API) probeTokenAuth(w http.ResponseWriter, r *http.Request) (Authentication, error) {
	// try logging in by probe token header
	credentials, ok := ParseAuthorizationHeader(r)
	if !ok || credentials.Realm != "Scope-Probe" {
		return Authentication{}, users.ErrInvalidAuthenticationData
	}

	token, ok := credentials.Params["token"]
	if !ok {
		return Authentication{}, users.ErrInvalidAuthenticationData
	}

	o, err := a.db.FindOrganizationByProbeToken(r.Context(), token)
	if err != nil {
		return Authentication{}, err
	}

	return Authentication{
		AuthType:      "probe_token",
		Organizations: []*users.Organization{o},
	}, nil
}
