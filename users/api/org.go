package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/billing/provider"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/orgs"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/common/validation"
	"github.com/weaveworks/service/users"
)

var errTeamIdentifierRequired = users.NewMalformedInputError(errors.New("either teamId or teamName needs to be provided but not both"))

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
	// Deprecated. Use TeamExternalID.
	TeamID         string `json:"teamId,omitempty"`         // TODO(rndstr): remove this once ui-server uses `teamId` as external
	TeamExternalID string `json:"teamExternalId,omitempty"` // TODO(rndstr): rename json output to `teamId`
	TeamName       string `json:"teamName,omitempty"`
}

// UnmarshalJSON does some postprocessing to support `teamExternalId` in JSON usage temporarily.
// TODO(rndstr): remove once ui-server` uses `teamId`.
func (o *OrgView) UnmarshalJSON(b []byte) error {
	// Prevent recursive loop
	type alias *OrgView
	var tmp alias = o
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}
	// For now we support teamId as well as teamExternalId to pass in the external id.
	// Internally we only use the variable TeamExternalID
	if tmp.TeamExternalID == "" {
		tmp.TeamExternalID = tmp.TeamID
	}
	return nil
}

func (a *API) org(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	organizations, err := a.db.ListOrganizationsForUserIDs(r.Context(), currentUser.ID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	for _, org := range organizations {
		if org.ExternalID == strings.ToLower(orgExternalID) {
			render.JSON(w, http.StatusOK, a.createOrgView(currentUser, org))
			return
		}
	}

	// If the organization exists but we just don't have access to it, tell the client
	if exists, err := a.db.OrganizationExists(r.Context(), orgExternalID); err != nil {
		renderError(w, r, err)
		return
	} else if exists {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	renderError(w, r, users.ErrNotFound)
}

func (a *API) createOrgView(currentUser *users.User, org *users.Organization) OrgView {
	return OrgView{
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
		TeamExternalID:       org.TeamExternalID,
		TeamID:               org.TeamExternalID,
	}
}

func (a *API) generateOrgExternalID(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	externalID, err := a.db.GenerateOrganizationExternalID(r.Context())
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, OrgView{Name: externalID, ExternalID: externalID})
}

func (a *API) createOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view OrgView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		renderError(w, r, users.NewMalformedInputError(err))
		return
	}
	// Don't allow users to specify their own token.
	view.ProbeToken = ""
	view.TrialExpiresAt = currentUser.TrialExpiresAt()
	if err := a.CreateOrg(r.Context(), currentUser, view, time.Now()); err == users.ErrOrgTokenIsTaken {
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if err != nil {
		renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// CreateOrg creates an organisation
func (a *API) CreateOrg(ctx context.Context, currentUser *users.User, view OrgView, now time.Time) error {
	if err := verifyTeamParams(view.TeamExternalID, view.TeamName); err != nil {
		return err
	}
	org, err := a.db.CreateOrganizationWithTeam(
		ctx,
		currentUser.ID,
		view.ExternalID,
		view.Name,
		view.ProbeToken,
		view.TeamExternalID,
		view.TeamName,
		view.TrialExpiresAt,
	)
	if err != nil {
		return err
	}

	return a.afterOrganizationCreatedOrMoved(ctx, currentUser, org, now)
}

func (a *API) updateOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var update users.OrgWriteView
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		renderError(w, r, users.NewMalformedInputError(err))
		return
	}
	orgExternalID := mux.Vars(r)["orgExternalID"]

	// Update.
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		renderError(w, r, err)
		return
	}
	org, err := a.db.UpdateOrganization(r.Context(), orgExternalID, update)
	if err != nil {
		renderError(w, r, err)
		return
	}

	// Move teams?
	if update.TeamExternalID != nil || update.TeamName != nil {
		if err := a.MoveOrg(r.Context(), currentUser, org, ptostr(update.TeamExternalID), ptostr(update.TeamName)); err != nil {
			renderError(w, r, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// MoveOrg moves the given organization to either an existing or a new team.
func (a *API) MoveOrg(ctx context.Context, currentUser *users.User, org *users.Organization, teamExternalID, teamName string) error {
	if teamExternalID != "" {
		if err := a.userCanAccessTeam(ctx, currentUser, teamExternalID); err != nil {
			return err
		}
	}

	if err := verifyTeamParams(teamExternalID, teamName); err != nil {
		return err
	}

	// Move organization into other team
	prevID := org.TeamID
	if err := a.db.MoveOrganizationToTeam(ctx, org.ExternalID, teamExternalID, teamName, currentUser.ID); err != nil {
		return err
	}

	// Update billing feature flags because the previous team might was externally billed and the new one isn't.
	// In this case, we need to reinstate the `billing` flag to prevent freeloading. The opposite direction—from
	// externally billed to locally billed—also needs a billing flag toggle.

	neworg, err := a.db.FindOrganizationByID(ctx, org.ExternalID)
	if err != nil {
		return err
	}

	bacc, err := a.billingClient.FindBillingAccountByTeamID(ctx, &grpc.BillingAccountByTeamIDRequest{
		TeamID: prevID,
	})
	if err != nil {
		return err
	}
	newbacc, err := a.billingClient.FindBillingAccountByTeamID(ctx, &grpc.BillingAccountByTeamIDRequest{
		TeamID: neworg.TeamID,
	})
	if err != nil {
		return err
	}

	// Only trigger restrictions update if we move from externally billed account to Zuora-billed account and vice versa
	if bacc.Provider != newbacc.Provider {
		if err := a.afterOrganizationCreatedOrMoved(ctx, currentUser, neworg, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

func verifyTeamParams(teamExternalID, teamName string) error {
	if (teamExternalID == "" && teamName == "") || (teamExternalID != "" && teamName != "") {
		return errTeamIdentifierRequired
	}
	return nil
}

// afterOrganizationSave is supposed to be an event hook that post processes the organixation after it has been created
// or updated. It makes sure the proper feature flags with regard to billing are set and also the data access/upload
// restrictions are in place.
func (a *API) afterOrganizationCreatedOrMoved(ctx context.Context, currentUser *users.User, org *users.Organization, now time.Time) error {
	var addFlag, otherFlag string

	if org.TeamID != "" {
		account, err := a.billingClient.FindBillingAccountByTeamID(ctx, &grpc.BillingAccountByTeamIDRequest{
			TeamID: org.TeamID,
		})
		if err != nil {
			return err
		}
		if account.Provider == provider.External {
			// No billing for instances billed externally.
			addFlag = featureflag.NoBilling
			otherFlag = featureflag.Billing
			log.Infof("Disabling billing for %v/%v/%v as team %v is billed externally.", org.ID, org.ExternalID, org.Name, org.TeamID)
		}
	}
	if addFlag == "" && a.billingEnabler.IsEnabled() {
		addFlag = featureflag.Billing
		otherFlag = featureflag.NoBilling
		log.Infof("Enabling billing for %v/%v/%v.", org.ID, org.ExternalID, org.Name)
	}

	if strings.HasSuffix(currentUser.Email, "@weave.works") {
		// No billing for instances created by Weaveworks people.
		addFlag = featureflag.NoBilling
		otherFlag = featureflag.Billing
		log.Infof("Disabling billing for %v/%v/%v as %v is a Weaver.", org.ID, org.ExternalID, org.Name, currentUser.Email)
	}

	if addFlag != "" {
		var ff []string
		for _, f := range org.FeatureFlags {
			if otherFlag != f && addFlag != f {
				ff = append(ff, f)
			}
		}
		ff = append(ff, addFlag)
		if err := a.db.SetFeatureFlags(ctx, org.ExternalID, ff); err != nil {
			return err
		}
		org.FeatureFlags = ff
	}

	if orgs.ShouldRefuseDataAccess(*org, now) {
		if err := a.db.SetOrganizationRefuseDataAccess(ctx, org.ExternalID, true); err != nil {
			log.Errorf("failed refusing data access for %s: %v", org.ExternalID, err)
			// do not return error, this is not crucial
		}
	}
	if orgs.ShouldRefuseDataUpload(*org, now) {
		if err := a.db.SetOrganizationRefuseDataUpload(ctx, org.ExternalID, true); err != nil {
			log.Errorf("failed refusing data upload for %s: %v", org.ExternalID, err)
			// do not return error, this is not crucial
		}
	}

	return nil
}

func ptostr(pstr *string) string {
	if pstr == nil {
		return ""
	}
	return *pstr
}

func (a *API) deleteOrg(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	orgExternalID := mux.Vars(r)["orgExternalID"]
	exists, err := a.db.OrganizationExists(r.Context(), orgExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	isMember, err := a.db.UserIsMemberOf(r.Context(), currentUser.ID, orgExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	if !isMember {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if err := a.db.DeleteOrganization(r.Context(), orgExternalID); err != nil {
		renderError(w, r, err)
		return
	}
	if a.OrgCleaner != nil {
		a.OrgCleaner.Trigger()
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
		renderError(w, r, err)
		return
	}

	users, err := a.db.ListOrganizationUsers(r.Context(), orgExternalID)
	if err != nil {
		renderError(w, r, err)
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
		renderError(w, r, users.NewMalformedInputError(err))
		return
	}
	email := strings.TrimSpace(resp.Email)
	if email == "" || !validation.ValidateEmail(email) {
		renderError(w, r, users.ErrEmailIsInvalid)
		return
	}

	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		renderError(w, r, err)
		return
	}

	invitee, created, err := a.db.InviteUser(r.Context(), email, orgExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := a.generateUserToken(r.Context(), invitee)
	if err != nil {
		renderError(w, r, fmt.Errorf("cannot send invite email: %s", err))
		return
	}
	orgName, err := a.db.GetOrganizationName(r.Context(), orgExternalID)
	if err != nil {
		renderError(w, r, fmt.Errorf("cannot get organization name: %s", err))
	}

	if created {
		err = a.emailer.InviteEmail(currentUser, invitee, orgExternalID, orgName, token)
	} else {
		err = a.emailer.GrantAccessEmail(currentUser, invitee, orgExternalID, orgName)
	}
	if err != nil {
		renderError(w, r, fmt.Errorf("cannot send invite email: %s", err))
		return
	}

	render.JSON(w, http.StatusOK, resp)
}

func (a *API) removeUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	userEmail := vars["userEmail"]
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		renderError(w, r, err)
		return
	}

	if members, err := a.db.ListOrganizationUsers(r.Context(), orgExternalID); err != nil {
		renderError(w, r, err)
		return
	} else if len(members) == 1 {
		// An organization cannot be with zero members
		renderError(w, r, users.ErrForbidden)
		return
	}

	if err := a.db.RemoveUserFromOrganization(r.Context(), orgExternalID, userEmail); err != nil {
		renderError(w, r, err)
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

func (a *API) userCanAccessTeam(ctx context.Context, currentUser *users.User, teamExternalID string) error {
	teams, err := a.db.ListTeamsForUserID(ctx, currentUser.ID)
	if err != nil {
		return err
	}
	for _, t := range teams {
		if t.ExternalID == teamExternalID {
			return nil
		}
	}
	return users.ErrForbidden
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
	enablingBilling, org, err := a.enablingOrgFeatureFlag(ctx, orgExternalID, uniqueFlags, featureflag.Billing)
	if err != nil {
		return err
	}

	if err = a.db.SetFeatureFlags(ctx, orgExternalID, sortedFlags); err != nil {
		return err
	}

	if enablingBilling {
		log.Infof("billing manually enabled for %v", orgExternalID)
		// For existing customers, we extend the trial period starting today and send members an email if we did so
		expires := time.Now().Add(users.TrialExtensionDuration)
		if err := a.extendOrgTrialPeriod(ctx, org, expires, true); err != nil {
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
func (a *API) extendOrgTrialPeriod(ctx context.Context, org *users.Organization, t time.Time, sendmail bool) error {
	// If new expiry date is not after current, there is nothing to change
	if !t.Truncate(24 * time.Hour).After(org.TrialExpiresAt) {
		return nil
	}
	logging.With(ctx).Infof("Extending trial period from %v to %v for %v", org.TrialExpiresAt, t, org.ExternalID)

	if _, err := a.db.UpdateOrganization(ctx, org.ExternalID, users.OrgWriteView{
		TrialExpiresAt:         &t,
		TrialExpiredNotifiedAt: &time.Time{}, // sets it to NULL
	}); err != nil {
		return err
	}

	if err := a.db.SetOrganizationRefuseDataAccess(ctx, org.ExternalID, false); err != nil {
		return err
	}
	if err := a.db.SetOrganizationRefuseDataUpload(ctx, org.ExternalID, false); err != nil {
		return err
	}

	if !sendmail {
		return nil
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

type orgLookupView struct {
	Name       string `json:"name"`
	ExternalID string `json:"externalID"`
}

func (a *API) orgLookup(org *users.Organization, w http.ResponseWriter, r *http.Request) {
	view := orgLookupView{Name: org.Name, ExternalID: org.ExternalID}
	render.JSON(w, http.StatusOK, view)
}
