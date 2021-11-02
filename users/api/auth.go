package api

import (
	"net/http"
	"time"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/tokens"
)

// Credentials are what gets parsed from ParseAuthorizationHeader
type Credentials struct {
	Realm  string
	Params map[string]string
}

// authenticateUser authenticates a user, passing that directly to the handler
func (a *API) authenticateUser(handler func(*users.User, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return a.authenticateUserVia(
		handler,
		UserAuthenticator(a.cookieAuth),
	)
}

// authenticateProbe tries authenticating via all known methods
func (a *API) authenticateProbe(handler func(*users.Organization, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return a.authenticateInstanceVia(
		handler,
		OrganizationAuthenticator(a.probeTokenAuth),
	)
}

// authenticateWebhook authenticates a request by a matching a header against a set of known tokens
func (a *API) authenticateWebhook(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("x-webhook-token")
		_, ok := a.webhookTokens[token]
		if ok {
			handler(w, r)
		} else {
			renderError(w, r, users.ErrForbidden)
		}
	}
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
		renderError(w, r, err)
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
		renderError(w, r, err)
		return
	})
}

func (a *API) cookieAuth(w http.ResponseWriter, r *http.Request) (*users.User, error) {
	// try logging in by cookie
	session, err := a.sessions.Get(r)
	if err != nil {
		return nil, err
	}
	u, err := a.db.FindUserByID(r.Context(), session.UserID)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (a *API) probeTokenAuth(w http.ResponseWriter, r *http.Request) (*users.Organization, error) {
	token, ok := tokens.ExtractToken(r)
	if !ok {
		return nil, users.ErrInvalidAuthenticationData
	}

	o, err := a.db.FindOrganizationByProbeToken(r.Context(), token)
	if err != nil {
		return nil, err
	}

	return o, nil
}
