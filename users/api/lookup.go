package api

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
	"github.com/weaveworks/service/users/sessions"
)

type lookupOrgRequest struct {
	Cookie, OrgExternalID string
}

type lookupOrgResponse struct {
	OrganizationID string   `json:"organizationID,omitempty"`
	UserID         string   `json:"userID,omitempty"`
	FeatureFlags   []string `json:"featureFlags,omitempty"`
}

func (a *API) lookupOrgHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := sessions.Extract(r)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	view, err := a.lookupOrg(r.Context(), &lookupOrgRequest{cookie, orgExternalID})
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

func (a *API) lookupOrg(ctx context.Context, req *lookupOrgRequest) (*lookupOrgResponse, error) {
	session, err := a.sessions.Decode(req.Cookie)
	if err != nil {
		return nil, err
	}

	organizations, err := a.db.ListOrganizationsForUserIDs(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	for _, org := range organizations {
		if strings.ToLower(org.ExternalID) == strings.ToLower(req.OrgExternalID) {
			return &lookupOrgResponse{
				OrganizationID: org.ID,
				UserID:         session.UserID,
				FeatureFlags:   org.FeatureFlags,
			}, nil
		}
	}
	return nil, users.ErrInvalidAuthenticationData
}

type lookupAdminView struct {
	AdminID string `json:"adminID,omitempty"`
}

func (a *API) lookupAdmin(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	if currentUser.Admin {
		render.JSON(w, http.StatusOK, lookupAdminView{AdminID: currentUser.ID})
		return
	}
	w.WriteHeader(http.StatusUnauthorized)
}

func (a *API) lookupUsingToken(organization *users.Organization, w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, lookupOrgResponse{OrganizationID: organization.ID, FeatureFlags: organization.FeatureFlags})
}

type lookupUserView struct {
	UserID string `json:"userID,omitempty"`
}

func (a *API) lookupUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, lookupUserView{UserID: currentUser.ID})
}
