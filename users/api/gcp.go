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

func requestHost(r *http.Request) string {
	scheme := r.URL.Scheme
	if scheme == "" {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
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
	sub, err := a.verifySubscriptionAccess(r.Context(), subName, input.Code, requestHost(r))
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
	gcp, err := a.db.CreateGCP(r.Context(), input.GCPAccountID, consumerID, sub.Name, level)
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

// verifySubscriptionAccess reads the subscription with the given oauth2 code.
// It also returns the received subscription.
func (a *API) verifySubscriptionAccess(ctx context.Context, name, oauthCode, host string) (*partner.Subscription, error) {
	// TODO(rndstr): put these into a config file
	cfg := oauth2.Config{
		ClientID:     "323808423072-m0nrvqj6h5k302gp55mej4nq0k1ossij.apps.googleusercontent.com",
		ClientSecret: "LWsytKd4BXhGHrMxoX4n0d7V",
		Scopes:       []string{"https://www.googleapis.com/auth/cloud-billing-partner-subscriptions.readonly"},
		Endpoint:     google.Endpoint,
		// Must be same URL as sent for code request
		RedirectURL: fmt.Sprintf("%s/subscribe-via/gcp", host),
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
		if sub.Status == partner.Pending {
			return sub.Name, nil
		}
	}
	return "", fmt.Errorf("no pending subscription found for account: %v", gcpAccountID)
}
