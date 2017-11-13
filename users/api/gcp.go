package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

type subscribeRequest struct {
	Code         string `json:"code"`
	GCPAccountID string `json:"gcpAccountId"`
}

func organizationName(externalID string) string {
	return strings.Title(strings.Replace(externalID, "-", " ", -1))
}

func (a *API) gcpAccess(w http.ResponseWriter, r *http.Request) {
	link, ok := a.partnerAccess.Link(r)
	if !ok {
		render.Error(w, r, errors.New("invalid token"))
	}
	render.JSON(w, http.StatusOK, link)
}

func (a *API) gcpSubscribe(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	state, ok := a.partnerAccess.VerifyState(r)
	if !ok {
		render.Error(w, r, errors.New("oauth state value did not match"))
	}
	gcpAccountID := state["gcpAccountId"]

	subName, err := a.getPendingSubscriptionName(r.Context(), gcpAccountID)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	sub, err := a.partnerAccess.RequestSubscription(r.Context(), r, subName)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	externalID, err := a.db.GenerateOrganizationExternalID(r.Context())
	if err != nil {
		render.Error(w, r, err)
		return
	}
	org, err := a.db.CreateOrganization(r.Context(), currentUser.ID, externalID, organizationName(externalID), "")
	if err != nil {
		render.Error(w, r, err)
		return
	}

	// Create and attach inactive GCP subscription to the organization
	level := sub.ExtractResourceLabel("weave-cloud", partner.ServiceLevelLabelKey)
	consumerID := sub.ExtractResourceLabel("weave-cloud", partner.ConsumerIDLabelKey)
	gcp, err := a.db.CreateGCP(r.Context(), gcpAccountID, consumerID, sub.Name, level)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	err = a.db.SetOrganizationGCP(r.Context(), org.ExternalID, gcp.AccountID)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	// Approve subscription: currently disabled to not "waste" the manually created subscription (approval can't be reversed)
	/*
		body := partner.RequestBodyWithSSOLoginKey(gcp.AccountID)
		_, err = a.partner.ApproveSubscription(r.Context(), sub.Name, body)
		if err != nil {
			render.Error(w, r, err)
			return
		}
	*/

	// Activate subscription account
	err = a.db.UpdateGCP(r.Context(), gcp.AccountID, gcp.ConsumerID, gcp.SubscriptionName, gcp.SubscriptionLevel, true)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, org)
}

func (a *API) getPendingSubscriptionName(ctx context.Context, gcpAccountID string) (string, error) {
	subs, err := a.partner.ListSubscriptions(ctx, gcpAccountID)
	if err != nil {
		return "", err
	}
	for _, sub := range subs {
		if sub.Status == partner.Pending {
			return sub.Name, nil
		}
	}
	return "", fmt.Errorf("no pending subscription found for account: %v", gcpAccountID)
}
