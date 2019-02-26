package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/weaveworks/service/common/gcp"
	"github.com/weaveworks/service/common/gcp/procurement"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

// We do not approve entitlements coming from this accountID as to not
// "waste" all of our staging billing accounts.
const testingExternalAccountID = "E-8A9F-4E2B-32D7-AC96"

var (
	// ErrAlreadyActivated says the account has already been activated and cannot be activated a second time.
	ErrAlreadyActivated = errors.New("account has already been activated")
)

func (a *API) gcpSSOLogin(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	claims, err := gcp.ParseJWT(r.FormValue("x-gcp-marketplace-token"))
	if err != nil {
		renderError(w, r, err)
		return
	}
	externalAccountID := claims.Subject
	org, err := a.GCPSSOLogin(currentUser, externalAccountID, w, r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, org)
}

// GCPSSOLogin attaches users logging in via GCP SSO to the organization attached to the provided externalAccountID.
// Behavior should be similar to regular invites to a specific instance.
func (a *API) GCPSSOLogin(currentUser *users.User, externalAccountID string, w http.ResponseWriter, r *http.Request) (*users.Organization, error) {
	logger := log.WithFields(log.Fields{"user_id": currentUser.ID, "email": currentUser.Email, "external_account_id": externalAccountID})
	logger.Infof("User SSO-ing into GCP account")
	org, err := a.db.FindOrganizationByGCPExternalAccountID(r.Context(), externalAccountID)
	if err != nil {
		return nil, err
	}
	if ok, err := a.db.UserIsMemberOf(r.Context(), currentUser.ID, org.ExternalID); err != nil {
		return nil, err
	} else if ok {
		logger.Infof("User already has access to organization [%v]", org.ExternalID)
		return org, nil
	}
	log.Infof("Now granting user %v access to organization [%v]", users.DefaultRoleID, org.ExternalID)
	if _, _, err = a.db.InviteUserToTeam(r.Context(), currentUser.Email, org.TeamExternalID, users.DefaultRoleID); err != nil {
		return nil, err
	}
	return org, nil
}

func (a *API) gcpSubscribe(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	claims, err := gcp.ParseJWT(r.FormValue("x-gcp-marketplace-token"))
	if err != nil {
		renderError(w, r, err)
		return
	}
	externalAccountID := claims.Subject

	org, err := a.GCPSubscribe(currentUser, externalAccountID, w, r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, org)
}

// GCPSubscribe creates an organization with GCP subscription. It also approves the subscription.
func (a *API) GCPSubscribe(currentUser *users.User, externalAccountID string, w http.ResponseWriter, r *http.Request) (*users.Organization, error) {
	logger := log.WithFields(log.Fields{"user_id": currentUser.ID, "email": currentUser.Email, "external_account_id": externalAccountID})
	logger.Info("Subscribing GCP Cloud Launcher user")

	ent, err := a.getPendingEntitlement(r.Context(), logger, externalAccountID)
	if err != nil {
		return nil, err
	}
	logger.Infof("Pending entitlement: %+v", ent)

	// Are we resuming?
	org, err := a.db.FindOrganizationByGCPExternalAccountID(r.Context(), externalAccountID)
	if err == users.ErrNotFound {
		// Nope, create a new instance.
		org, err = a.db.CreateOrganizationWithGCP(r.Context(), currentUser.ID, externalAccountID, currentUser.TrialExpiresAt())
	}
	if err != nil {
		return nil, err
	}
	if externalAccountID != testingExternalAccountID {
		if org.GCP.Activated {
			return nil, ErrAlreadyActivated
		}

		if err := a.procurement.ApproveAccount(r.Context(), externalAccountID); err != nil {
			return nil, err
		}

		if err := a.procurement.ApproveEntitlement(r.Context(), ent.Name, ""); err != nil {
			return nil, err
		}
	}

	// Mark GCP account as activated account.
	// Set subscription status to ACTIVE because approval passed. We will also receive a PubSub message
	// with Status = ACTIVE later. If we set it to PENDING here (what sub.Status currently is), we get
	// into a race with the PubSub message.
	if err = a.db.UpdateGCP(r.Context(), externalAccountID, ent.UsageReportingID, ent.Name, ent.Plan,
		string(procurement.Active)); err != nil {
		return nil, err
	}

	return org, nil
}

func (a *API) adminGCPListEntitlements(w http.ResponseWriter, r *http.Request) {
	externalAccountID := mux.Vars(r)["externalAccountID"]
	ents, err := a.procurement.ListEntitlements(r.Context(), externalAccountID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, ents)
}

func (a *API) getPendingEntitlement(ctx context.Context, logger *log.Entry, externalAccountID string) (*procurement.Entitlement, error) {
	ents, err := a.procurement.ListEntitlements(ctx, externalAccountID)
	if err != nil {
		return nil, errors.Wrap(err, "cannot list entitlements")
	}
	logger.Infof("Received entitlements: %+v", ents)
	for _, e := range ents {
		if e.State == procurement.ActivationRequested {
			return &e, nil
		}
	}
	// In case our test account no longer has a PENDING subscription,
	// try to re-use an ACTIVE subscription, and if none,
	// fallback on the first subscription found, even if COMPLETE:
	if externalAccountID == testingExternalAccountID {
		return findTestEntitlement(ents)
	}
	return nil, users.NewMalformedInputError(fmt.Errorf("no pending entitlement found"))
}

func findTestEntitlement(ents []procurement.Entitlement) (*procurement.Entitlement, error) {
	for _, e := range ents {
		if e.State == procurement.Active {
			return &e, nil
		}
	}
	return &ents[0], nil
}

func (a *API) getGoogleOAuthToken(ctx context.Context, logger *log.Entry, userID string) (*oauth2.Token, error) {
	logins, err := a.db.ListLoginsForUserIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, l := range logins {
		if l.Provider == login.GoogleProviderID {
			var session login.OAuthUserSession
			if err := json.Unmarshal(l.Session, &session); err != nil {
				return nil, err
			}
			return session.Token, nil
		}
	}
	// "no active Google OAuth session, please authenticate again"
	return nil, users.ErrInvalidAuthenticationData
}
