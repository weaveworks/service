package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

// OrgView describes an organisation
type OrgView struct {
	User                  string     `json:"user,omitempty"`
	ExternalID            string     `json:"id"`
	Name                  string     `json:"name"`
	ProbeToken            string     `json:"probeToken,omitempty"`
	FeatureFlags          []string   `json:"featureFlags,omitempty"`
	DenyUIFeatures        bool       `json:"denyUIFeatures"`
	DenyTokenAuth         bool       `json:"denyTokenAuth"`
	FirstSeenConnectedAt  *time.Time `json:"firstSeenConnectedAt"`
	Platform              string     `json:"platform"`
	Environment           string     `json:"environment"`
	TrialExpiresAt        time.Time  `json:"trialExpiresAt"`
	ZuoraAccountNumber    string     `json:"zuoraAccountNumber"`
	ZuoraAccountCreatedAt *time.Time `json:"zuoraAccountCreatedAt"`
}

func (a *API) org(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	organizations, err := a.db.ListOrganizationsForUserIDs(r.Context(), currentUser.ID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	for _, org := range organizations {
		if strings.ToLower(org.ExternalID) == strings.ToLower(orgExternalID) {
			render.JSON(w, http.StatusOK, OrgView{
				User:                 currentUser.Email,
				ExternalID:           org.ExternalID,
				Name:                 org.Name,
				ProbeToken:           org.ProbeToken,
				FeatureFlags:         append(org.FeatureFlags, a.forceFeatureFlags...),
				DenyUIFeatures:       org.DenyUIFeatures,
				DenyTokenAuth:        org.DenyTokenAuth,
				FirstSeenConnectedAt: org.FirstSeenConnectedAt,
				Platform:             org.Platform,
				Environment:          org.Environment,
				TrialExpiresAt:       org.TrialExpiresAt,
			})
			return
		}
	}
	if exists, err := a.db.OrganizationExists(r.Context(), orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	} else if exists {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	render.Error(w, r, users.ErrNotFound)
}

func (a *API) generateOrgExternalID(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	externalID, err := a.db.GenerateOrganizationExternalID(r.Context())
	if err != nil {
		render.Error(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, OrgView{Name: externalID, ExternalID: externalID})
}

func (a *API) createOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view OrgView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		render.Error(w, r, users.MalformedInputError(err))
		return
	}
	// Don't allow users to specify their own token.
	view.ProbeToken = ""
	if err := a.CreateOrg(r.Context(), currentUser, view); err == users.ErrOrgTokenIsTaken {
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// CreateOrg creates an organisation
func (a *API) CreateOrg(ctx context.Context, currentUser *users.User, view OrgView) error {
	_, err := a.db.CreateOrganization(ctx, currentUser.ID, view.ExternalID, view.Name, view.ProbeToken)
	return err
}

func (a *API) updateOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var update users.OrgWriteView
	err := json.NewDecoder(r.Body).Decode(&update)
	switch {
	case err != nil:
		render.Error(w, r, users.MalformedInputError(err))
		return
	}
	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}
	if err := a.db.UpdateOrganization(r.Context(), orgExternalID, update); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) deleteOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	orgExternalID := mux.Vars(r)["orgExternalID"]
	exists, err := a.db.OrganizationExists(r.Context(), orgExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	isMember, err := a.db.UserIsMemberOf(r.Context(), currentUser.ID, orgExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	if !isMember {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if err := a.db.DeleteOrganization(r.Context(), orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type organizationUsersView struct {
	Users []organizationUserView `json:"users"`
}

type organizationUserView struct {
	Email string `json:"email"`
	Self  bool   `json:"self,omitempty"`
}

func (a *API) listOrganizationUsers(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}

	users, err := a.db.ListOrganizationUsers(r.Context(), orgExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	view := organizationUsersView{}
	for _, u := range users {
		view.Users = append(view.Users, organizationUserView{
			Email: u.Email,
			Self:  u.ID == currentUser.ID,
		})
	}
	render.JSON(w, http.StatusOK, view)
}

func (a *API) inviteUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view SignupView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		render.Error(w, r, users.MalformedInputError(err))
		return
	}
	view.MailSent = false
	if view.Email == "" {
		render.Error(w, r, users.ValidationErrorf("Email cannot be blank"))
		return
	}

	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}

	invitee, created, err := a.db.InviteUser(r.Context(), view.Email, orgExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := a.generateUserToken(r.Context(), invitee)
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	orgName, err := a.db.GetOrganizationName(r.Context(), orgExternalID)
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error getting organization name: %s", err))
	}

	if created {
		err = a.emailer.InviteEmail(currentUser, invitee, orgExternalID, orgName, token)
	} else {
		err = a.emailer.GrantAccessEmail(currentUser, invitee, orgExternalID, orgName)
	}
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error sending invite email: %s", err))
		return
	}
	view.MailSent = true

	render.JSON(w, http.StatusOK, view)
}

func (a *API) deleteUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	userEmail := vars["userEmail"]
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}

	if err := a.db.RemoveUserFromOrganization(r.Context(), orgExternalID, userEmail); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) userCanAccessOrg(ctx context.Context, currentUser *users.User, orgExternalID string) error {
	isMember, err := a.db.UserIsMemberOf(ctx, currentUser.ID, orgExternalID)
	if err != nil {
		return err
	}
	if !isMember {
		if exists, err := a.db.OrganizationExists(ctx, orgExternalID); err != nil {
			return err
		} else if exists {
			return users.ErrForbidden
		}
		return users.ErrNotFound
	}
	return nil
}
