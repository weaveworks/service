package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

// We do not approve subscriptions coming from this accountID as to not
// "waste" all of our staging billing accounts.
const testingAccountID = "E-97A7-79FC-AD2D-9D31"

func (a *API) gcpAccess(w http.ResponseWriter, r *http.Request) {
	link, ok := a.partnerAccess.Link(r)
	if !ok {
		render.Error(w, r, errors.New("invalid token"))
		return
	}
	render.JSON(w, http.StatusOK, link)
}

func (a *API) gcpSSOLogin(w http.ResponseWriter, r *http.Request) {
	gcpAccountID := path.Base(r.URL.Path)

	org, err := a.db.FindOrganizationByGCPAccountID(r.Context(), gcpAccountID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	admins, err := a.db.ListOrganizationUsers(r.Context(), org.ExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	user := admins[0] // Arbitrarily log in as first admin.

	firstLogin := user.FirstLoginAt.IsZero()
	if err := a.UpdateUserAtLogin(r.Context(), user); err != nil {
		render.Error(w, r, err)
		return
	}
	impersonatingUserID := "" // SSO login => cannot be impersonating
	if err := a.sessions.Set(w, r, user.ID, impersonatingUserID); err != nil {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
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
	state, ok := a.partnerAccess.VerifyState(r)
	if !ok {
		render.Error(w, r, errors.New("oauth state value did not match"))
		return
	}

	gcpAccountID := state["gcpAccountId"]
	logger := log.WithFields(log.Fields{"user_id": currentUser.ID, "email": currentUser.Email, "account_id": gcpAccountID})
	subName, err := a.getPendingSubscriptionName(r.Context(), logger, gcpAccountID)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	sub, err := a.partnerAccess.RequestSubscription(r.Context(), r, subName)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	logger.Infof("Pending subscription: %+v", sub)

	level := sub.ExtractResourceLabel("weave-cloud", partner.ServiceLevelLabelKey)
	consumerID := sub.ExtractResourceLabel("weave-cloud", partner.ConsumerIDLabelKey)
	if consumerID == "" {
		render.Error(w, r, errors.New("no consumer ID found"))
		return
	}
	org, gcp, err := a.db.CreateOrganizationWithGCP(r.Context(), currentUser.ID, gcpAccountID, consumerID, sub.Name, level)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	if gcpAccountID != testingAccountID {
		// Approve subscription
		body := partner.RequestBodyWithSSOLoginKey(gcp.AccountID)
		_, err = a.partner.ApproveSubscription(r.Context(), sub.Name, body)
		if err != nil {
			render.Error(w, r, err)
			return
		}
	}

	// Activate subscription account
	err = a.db.UpdateGCP(r.Context(), gcp.AccountID, gcp.ConsumerID, gcp.SubscriptionName, gcp.SubscriptionLevel, true)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, org)
}

func (a *API) getPendingSubscriptionName(ctx context.Context, logger *log.Entry, gcpAccountID string) (string, error) {
	subs, err := a.partner.ListSubscriptions(ctx, gcpAccountID)
	if err != nil {
		return "", err
	}
	logger.Infof("Received subscriptions: %+v", subs)
	for _, sub := range subs {
		if sub.Status == partner.Pending {
			return sub.Name, nil
		}
	}
	return "", fmt.Errorf("no pending subscription found for account: %v", gcpAccountID)
}
