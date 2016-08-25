package api

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

type lookupOrgView struct {
	OrganizationID string `json:"organizationID,omitempty"`
	UserID         string `json:"userID,omitempty"`
}

func (a *API) lookupOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	for _, org := range currentUser.Organizations {
		if strings.ToLower(org.ExternalID) == strings.ToLower(orgExternalID) {
			render.JSON(w, http.StatusOK, lookupOrgView{
				OrganizationID: org.ID,
				UserID:         currentUser.ID,
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

func (a *API) lookupUsingToken(w http.ResponseWriter, r *http.Request) {
	credentials, ok := ParseAuthHeader(r.Header.Get("Authorization"))
	if !ok || credentials.Realm != "Scope-Probe" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token, ok := credentials.Params["token"]
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	org, err := a.db.FindOrganizationByProbeToken(token)
	if err == nil {
		render.JSON(w, http.StatusOK, lookupOrgView{OrganizationID: org.ID})
		return
	}

	if err != users.ErrInvalidAuthenticationData {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		render.Error(w, r, err)
	}
}
