package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"

	"github.com/weaveworks/common/logging"
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
	RefuseDataAccess      bool       `json:"refuseDataAccess"`
	RefuseDataUpload      bool       `json:"refuseDataUpload"`
	FirstSeenConnectedAt  *time.Time `json:"firstSeenConnectedAt"`
	Platform              string     `json:"platform"`
	Environment           string     `json:"environment"`
	TrialExpiresAt        time.Time  `json:"trialExpiresAt"`
	ZuoraAccountNumber    string     `json:"zuoraAccountNumber"`
	ZuoraAccountCreatedAt *time.Time `json:"zuoraAccountCreatedAt"`
	BillingProvider       string     `json:"billingProvider"`
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
		if org.ExternalID == strings.ToLower(orgExternalID) {
			render.JSON(w, http.StatusOK, OrgView{
				User:                 currentUser.Email,
				ExternalID:           org.ExternalID,
				Name:                 org.Name,
				ProbeToken:           org.ProbeToken,
				FeatureFlags:         append(org.FeatureFlags, a.forceFeatureFlags...),
				RefuseDataAccess:     org.RefuseDataAccess,
				RefuseDataUpload:     org.RefuseDataUpload,
				FirstSeenConnectedAt: org.FirstSeenConnectedAt,
				Platform:             org.Platform,
				Environment:          org.Environment,
				TrialExpiresAt:       org.TrialExpiresAt,
				BillingProvider:      org.BillingProvider(),
			})
			return
		}
	}

	// If the organization exists but we just don't have access to it, tell the client
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
	org, err := a.db.CreateOrganization(ctx, currentUser.ID, view.ExternalID, view.Name, view.ProbeToken)
	if err != nil {
		return err
	}
	if a.billingEnabler.IsEnabled() {
		err = a.db.AddFeatureFlag(ctx, view.ExternalID, users.BillingFeatureFlag)
		if err != nil {
			return err
		}
		log.Infof("Billing enabled for %v/%v/%v.", org.ID, view.ExternalID, view.Name)
	}
	return nil
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
	var resp SignupResponse
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		render.Error(w, r, users.MalformedInputError(err))
		return
	}
	if resp.Email == "" {
		render.Error(w, r, users.ValidationErrorf("Email cannot be blank"))
		return
	}

	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		render.Error(w, r, err)
		return
	}

	invitee, created, err := a.db.InviteUser(r.Context(), resp.Email, orgExternalID)
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

	render.JSON(w, http.StatusOK, resp)
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

// setOrgFeatureFlags updates feature flags of an organization.
func (a *API) setOrgFeatureFlags(ctx context.Context, orgExternalID string, flags []string) error {
	uniqueFlags := map[string]struct{}{}
	for _, f := range flags {
		uniqueFlags[f] = struct{}{}
	}
	var sortedFlags []string
	for f := range uniqueFlags {
		sortedFlags = append(sortedFlags, f)
	}
	sort.Strings(sortedFlags)

	// Track whether we are about to enable billing
	enablingBilling, org, err := a.enablingOrgFeatureFlag(ctx, orgExternalID, uniqueFlags, users.BillingFeatureFlag)
	if err != nil {
		return err
	}

	if err = a.db.SetFeatureFlags(ctx, orgExternalID, sortedFlags); err != nil {
		return err
	}

	if enablingBilling {
		log.Infof("Billing manually enabled for %v", orgExternalID)
		// For existing customers, we extend the trial period starting today and send members an email if we did so
		expires := time.Now().Add(users.TrialExtensionDuration)
		if err := a.extendOrgTrialPeriod(ctx, org, expires); err != nil {
			return err
		}
	}

	return nil
}

// enablingOrgFeatureFlag determines whether we are about to enable a feature flag for the given organization. It only
// returns true if the organization did *not* have the flag previously.
func (a *API) enablingOrgFeatureFlag(ctx context.Context, orgExternalID string, flags map[string]struct{}, flag string) (bool, *users.Organization, error) {
	if _, ok := flags[flag]; !ok {
		return false, nil, nil
	}
	org, err := a.db.FindOrganizationByID(ctx, orgExternalID)
	if err != nil {
		return false, nil, err
	}

	return !org.HasFeatureFlag(flag), org, nil
}

// extendOrgTrialPeriod update the trial period but only if it's later than the current.
// It also sends an email to all members notifying them of the change.
func (a *API) extendOrgTrialPeriod(ctx context.Context, org *users.Organization, t time.Time) error {
	// If new expiry date is not after current, there is nothing to change
	if !t.Truncate(24 * time.Hour).After(org.TrialExpiresAt) {
		return nil
	}
	logging.With(ctx).Infof("Extending trial period from %v to %v for %v", org.TrialExpiresAt, t, org.ExternalID)

	err := a.db.UpdateOrganization(ctx, org.ExternalID, users.OrgWriteView{TrialExpiresAt: &t})
	if err != nil {
		return err
	}

	members, err := a.db.ListOrganizationUsers(ctx, org.ExternalID)
	if err != nil {
		return err
	}

	err = a.emailer.TrialExtendedEmail(members, org.ExternalID, org.Name, t)
	if err != nil {
		return err
	}

	return nil
}
