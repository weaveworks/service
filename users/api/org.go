package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/billing/provider"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/orgs"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/common/validation"
	"github.com/weaveworks/service/notification-eventmanager/types"
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
	TeamExternalID        string     `json:"teamId,omitempty"`
	TeamName              string     `json:"teamName,omitempty"`
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

func (a *API) createEmailReceiver(ctx context.Context, email, orgID string) error {
	emailData, err := json.Marshal(email)
	if err != nil {
		return errors.Wrapf(err, "cannot marshal email address %s", email)
	}

	r := types.Receiver{
		RType:       types.EmailReceiver,
		AddressData: emailData,
	}

	data, err := json.Marshal(r)
	if err != nil {
		return errors.Wrapf(err, "cannot marshal receiver data %#v", r)
	}

	req, err := http.NewRequest("POST", a.notificationReceiversURL, bytes.NewBuffer(data))
	if err != nil {
		return errors.Wrap(err, "error constructing POST HTTP request to create email receiver")
	}

	ctxWithID := user.InjectOrgID(ctx, orgID)
	err = user.InjectOrgIDIntoHTTPRequest(ctxWithID, req)
	if err != nil {
		return errors.Wrap(err, "cannot inject org ID into request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP POST to notification service")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		return fmt.Errorf("%s from notification service (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil

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

	if err := a.refreshOrganizationRestrictions(ctx, currentUser, org, now); err != nil {
		return err
	}

	if err := a.createEmailReceiver(ctx, currentUser.Email, org.ID); err != nil {
		// Just warning because it's ok to create org without email receiver
		// by default browser receiver will be created for all orgs
		log.Warnf("cannot create email receiver with address %s, error: %s", currentUser.Email, err)
	}

	return nil
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
		if _, err := a.userCanAccessTeam(ctx, currentUser, teamExternalID); err != nil {
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
		if err := a.refreshOrganizationRestrictions(ctx, currentUser, neworg, time.Now()); err != nil {
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

// refreshOrganizationRestrictions updates restrictions of an organization with regard to billing.
// It makes sure the proper feature flags are set and also the data access/upload restrictions are in place.
func (a *API) refreshOrganizationRestrictions(ctx context.Context, currentUser *users.User, org *users.Organization, now time.Time) error {
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

			// Never refuse data for externally billed instances.
			if org.RefuseDataAccess {
				if err := a.db.SetOrganizationRefuseDataAccess(ctx, org.ExternalID, false); err != nil {
					log.Errorf("Failed refusing data access for %s: %v", org.ExternalID, err)
				}
			}
			if org.RefuseDataUpload {
				if err := a.db.SetOrganizationRefuseDataUpload(ctx, org.ExternalID, false); err != nil {
					log.Errorf("Failed refusing data access for %s: %v", org.ExternalID, err)
				}
			}
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
			log.Errorf("Failed refusing data access for %s: %v", org.ExternalID, err)
			// do not return error, this is not crucial
		}
	}
	if orgs.ShouldRefuseDataUpload(*org, now) {
		if err := a.db.SetOrganizationRefuseDataUpload(ctx, org.ExternalID, true); err != nil {
			log.Errorf("Failed refusing data upload for %s: %v", org.ExternalID, err)
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
	org, err := a.db.FindOrganizationByID(r.Context(), orgExternalID)
	if err == users.ErrNotFound {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		renderError(w, r, err)
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
	// GCP instances should always be disabled through Google
	if org.GCP != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if err := a.db.DeleteOrganization(r.Context(), orgExternalID); err != nil {
		renderError(w, r, err)
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
	user.LogWith(ctx, logging.Global()).Infof("Extending trial period from %v to %v for %v", org.TrialExpiresAt, t, org.ExternalID)

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
