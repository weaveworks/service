package api

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

type lookupOrgView struct {
	OrganizationID string   `json:"organizationID,omitempty"`
	UserID         string   `json:"userID,omitempty"`
	FeatureFlags   []string `json:"featureFlags,omitempty"`
}

func (a *API) lookupOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	organizations, err := a.db.ListOrganizationsForUserIDs(currentUser.ID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	for _, org := range organizations {
		if strings.ToLower(org.ExternalID) == strings.ToLower(orgExternalID) {
			render.JSON(w, http.StatusOK, lookupOrgView{
				OrganizationID: org.ID,
				UserID:         currentUser.ID,
				FeatureFlags:   org.FeatureFlags,
			})
			return
		}
	}
	render.Error(w, r, users.ErrNotFound)
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
	render.JSON(w, http.StatusOK, lookupOrgView{OrganizationID: organization.ID, FeatureFlags: organization.FeatureFlags})
}

type lookupUserView struct {
	UserID string `json:"userID,omitempty"`
}

func (a *API) lookupUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	render.JSON(w, http.StatusOK, lookupUserView{UserID: currentUser.ID})
}
