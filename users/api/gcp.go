package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

// We do not approve subscriptions coming from this accountID as to not
// "waste" all of our staging billing accounts.
const testingExternalAccountID = "E-97A7-79FC-AD2D-9D31"

var (
	// ErrAlreadyActivated says the account has already been activated and cannot be activated a second time.
	ErrAlreadyActivated = errors.New("account has already been activated")
	// ErrMissingConsumerID denotes the consumerId label is missing in the subscribed resource.
	ErrMissingConsumerID = errors.New("missing consumer ID")
)

func (a *API) gcpSSOLogin(w http.ResponseWriter, r *http.Request) {
	externalAccountID := mux.Vars(r)["externalAccountID"]

	org, err := a.db.FindOrganizationByGCPExternalAccountID(r.Context(), externalAccountID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	admins, err := a.db.ListOrganizationUsers(r.Context(), org.ExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	user := admins[0] // Arbitrarily log in as first admin.

	firstLogin := user.FirstLoginAt.IsZero()
	if err := a.UpdateUserAtLogin(r.Context(), user); err != nil {
		renderError(w, r, err)
		return
	}
	impersonatingUserID := "" // SSO login => cannot be impersonating
	if err := a.sessions.Set(w, r, user.ID, impersonatingUserID); err != nil {
		renderError(w, r, users.ErrInvalidAuthenticationData)
		return
	}
	// Track mixpanel event https://github.com/weaveworks/service/issues/1301
	if a.mixpanel != nil {
		go func() {
			if err := a.mixpanel.TrackLogin(user.Email, firstLogin); err != nil {
				logging.With(r.Context()).Error(err)
			}
		}()
	}
	http.Redirect(w, r, "/", 302)
}

func (a *API) gcpSubscribe(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	externalAccountID := r.FormValue("gcpAccountId")
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
	subName, err := a.getPendingSubscriptionName(r.Context(), logger, externalAccountID)
	if err != nil {
		return nil, err
	}

	token, err := a.getGoogleOAuthToken(r.Context(), logger, currentUser.ID)
	if err != nil {
		return nil, err
	}
	sub, err := a.partnerAccess.RequestSubscription(r.Context(), token, subName)
	if err != nil {
		return nil, err
	}
	logger.Infof("Pending subscription: %+v", sub)

	level := sub.ExtractResourceLabel("weave-cloud", partner.ServiceLevelLabelKey)
	consumerID := sub.ExtractResourceLabel("weave-cloud", partner.ConsumerIDLabelKey)
	if consumerID == "" {
		return nil, ErrMissingConsumerID
	}

	// Are we resuming?
	org, err := a.db.FindOrganizationByGCPExternalAccountID(r.Context(), externalAccountID)
	if err == users.ErrNotFound {
		// Nope, create a new instance.
		org, err = a.db.CreateOrganizationWithGCP(r.Context(), currentUser.ID, externalAccountID)
	}
	if err != nil {
		return nil, err
	}
	if externalAccountID != testingExternalAccountID {
		if org.GCP.Activated {
			return nil, ErrAlreadyActivated
		}

		// Approve subscription
		body := partner.RequestBodyWithSSOLoginKey(externalAccountID)
		_, err = a.partner.ApproveSubscription(r.Context(), sub.Name, body)
		if err != nil {
			return nil, err
		}
	}

	// Mark GCP account as activated account.
	// Set subscription status to ACTIVE because approval passed. We will also receive a PubSub message
	// with Status = ACTIVE later. If we set it to PENDING here (what sub.Status currently is), we get
	// into a race with the PubSub message.
	err = a.db.UpdateGCP(r.Context(), externalAccountID, consumerID, sub.Name, level, string(partner.Active))
	if err != nil {
		return nil, err
	}

	return org, nil
}

func (a *API) gcpListSubscriptions(w http.ResponseWriter, r *http.Request) {
	externalAccountID := mux.Vars(r)["externalAccountID"]
	subs, err := a.partner.ListSubscriptions(r.Context(), externalAccountID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, subs)
}

func (a *API) getPendingSubscriptionName(ctx context.Context, logger *log.Entry, externalAccountID string) (string, error) {
	subs, err := a.partner.ListSubscriptions(ctx, externalAccountID)
	if err != nil {
		return "", err
	}
	logger.Infof("Received subscriptions: %+v", subs)
	for _, sub := range subs {
		if sub.Status == partner.Pending {
			return sub.Name, nil
		}
	}
	err = fmt.Errorf("no pending subscription found for account: %v", externalAccountID)
	return "", users.NewMalformedInputError(err)
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
