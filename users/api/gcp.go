package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

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

func (a *API) subscribe(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		render.Error(w, r, users.MalformedInputError(err))
		return
	}

	subName, err := a.getPendingSubscriptionName(r.Context(), input.GCPAccountID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	sub, err := a.verifySubscriptionAccess(r.Context(), subName, input.Code)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	fmt.Printf("Received subscription: %#v\n", sub)

	externalID, err := a.db.GenerateOrganizationExternalID(r.Context())
	if err != nil {
		render.Error(w, r, err)
		return
	}
	// FIXME: expand CreateOrganization to also create the GCP (in a transaction)
	org, err := a.db.CreateOrganization(r.Context(), currentUser.ID, externalID, organizationName(externalID), "")
	if err != nil {
		render.Error(w, r, err)
		return
	}
	// Attach inactive GCP subscription to the organization
	level := sub.ExtractResourceLabel("weave-cloud", partner.ServiceLevelLabelKey)
	consumerID := sub.ExtractResourceLabel("weave-cloud", partner.ConsumerIDLabelKey)
	err = a.db.AddGCPToOrganization(r.Context(), org.ExternalID, input.GCPAccountID, consumerID, sub.Name, level)
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

	// Activate organization
	err = a.db.UpdateOrganizationGCP(r.Context(), org.ExternalID, consumerID, sub.Name, level)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, org)
}

func (a *API) verifySubscriptionAccess(ctx context.Context, name, oauthCode string) (*partner.Subscription, error) {
	cfg := oauth2.Config{
		ClientID:     "323808423072-m0nrvqj6h5k302gp55mej4nq0k1ossij.apps.googleusercontent.com",
		Scopes:       []string{"https://www.googleapis.com/auth/cloud-billing-partner-subscriptions.readonly"},
		ClientSecret: "LWsytKd4BXhGHrMxoX4n0d7V",
		Endpoint:     google.Endpoint,
		// Must be same URL as sent for code request
		RedirectURL: "http://localhost:4046/subscribe-via/gcp",
	}

	// Get token from oauth code
	token, err := cfg.Exchange(ctx, oauthCode)
	if err != nil {
		return nil, err
	}

	// Now verify that the user's token can actually access the subscription
	cl, err := partner.NewClientFromTokenSource(oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, err
	}
	sub, err := cl.GetSubscription(ctx, name)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (a *API) getPendingSubscriptionName(ctx context.Context, gcpAccountID string) (string, error) {
	subs, err := a.partner.ListSubscriptions(ctx, gcpAccountID)
	if err != nil {
		return "", err
	}
	fmt.Printf("%#v\n", subs)
	for _, sub := range subs {
		if sub.Status == partner.StatusPending {
			return sub.Name, nil
		}
	}
	return "", fmt.Errorf("no pending subscription found for account: %v", gcpAccountID)
}
