package api

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/tokens"
)

func (a *API) lookupOrgHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := sessions.Extract(r)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	orgExternalID := mux.Vars(r)["orgExternalID"]

	view, err := a.LookupOrg(r.Context(), &users.LookupOrgRequest{cookie, orgExternalID})
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

// LookupOrg authenticates a cookie for access to an org by extenal ID.
func (a *API) LookupOrg(ctx context.Context, req *users.LookupOrgRequest) (*users.LookupOrgResponse, error) {
	session, err := a.sessions.Decode(req.Cookie)
	if err != nil {
		return nil, err
	}

	organizations, err := a.db.ListOrganizationsForUserIDs(ctx, session.UserID)
	if err == users.ErrNotFound {
		err = users.ErrInvalidAuthenticationData
	}
	if err != nil {
		return nil, err
	}
	for _, org := range organizations {
		if strings.ToLower(org.ExternalID) == strings.ToLower(req.OrgExternalID) {
			return &users.LookupOrgResponse{
				OrganizationID: org.ID,
				UserID:         session.UserID,
				FeatureFlags:   org.FeatureFlags,
			}, nil
		}
	}
	return nil, users.ErrInvalidAuthenticationData
}

func (a *API) lookupAdminHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := sessions.Extract(r)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	view, err := a.LookupAdmin(r.Context(), &users.LookupAdminRequest{
		Cookie: cookie,
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

// LookupAdmin authenticates a cookie for admin access.
func (a *API) LookupAdmin(ctx context.Context, req *users.LookupAdminRequest) (*users.LookupAdminResponse, error) {
	session, err := a.sessions.Decode(req.Cookie)
	if err != nil {
		return nil, err
	}
	u, err := a.db.FindUserByID(ctx, session.UserID)
	if err == users.ErrNotFound {
		err = users.ErrInvalidAuthenticationData
	}
	if err != nil {
		return nil, err
	}
	if !u.Admin {
		return nil, users.ErrInvalidAuthenticationData
	}
	return &users.LookupAdminResponse{
		AdminID: u.ID,
	}, nil
}

func (a *API) lookupUsingTokenHandler(w http.ResponseWriter, r *http.Request) {
	token, ok := tokens.ExtractToken(r)
	if !ok {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	view, err := a.LookupUsingToken(r.Context(), &users.LookupUsingTokenRequest{
		Token: token,
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

// LookupUsingToken authenticates a token for access to an org.
func (a *API) LookupUsingToken(ctx context.Context, req *users.LookupUsingTokenRequest) (*users.LookupUsingTokenResponse, error) {
	o, err := a.db.FindOrganizationByProbeToken(ctx, req.Token)
	if err != nil {
		return nil, err
	}

	return &users.LookupUsingTokenResponse{
		OrganizationID: o.ID,
		FeatureFlags:   o.FeatureFlags,
	}, nil
}

func (a *API) lookupUserHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := sessions.Extract(r)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	view, err := a.LookupUser(r.Context(), &users.LookupUserRequest{
		Cookie: cookie,
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

// LookupUser authenticates a cookie.
func (a *API) LookupUser(ctx context.Context, req *users.LookupUserRequest) (*users.LookupUserResponse, error) {
	session, err := a.sessions.Decode(req.Cookie)
	if err != nil {
		return nil, err
	}
	return &users.LookupUserResponse{session.UserID}, nil
}
