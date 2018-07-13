package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/weaveworks/service/common/constants/webhooks"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
)

func (a *API) listOrganizationWebhooks(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		renderError(w, r, err)
		return
	}

	webhooks, err := a.db.ListOrganizationWebhooks(r.Context(), orgExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, webhooks)
}

type createOrganizationWebhookPayload struct {
	IntegrationType string `json:"integrationType"`
}

func (a *API) createOrganizationWebhook(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		renderError(w, r, err)
		return
	}

	defer r.Body.Close()
	var payload createOrganizationWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		renderError(w, r, users.NewMalformedInputError(err))
		return
	}

	// Validate integration type
	switch payload.IntegrationType {
	case webhooks.GithubPushIntegrationType:
		break
	default:
		renderError(w, r, users.NewMalformedInputError(fmt.Errorf("Invalid integration type")))
		return
	}

	webhook, err := a.db.CreateOrganizationWebhook(r.Context(), orgExternalID, payload.IntegrationType)
	if err != nil {
		renderError(w, r, err)
		return
	}

	render.JSON(w, http.StatusCreated, webhook)
}

func (a *API) deleteOrganizationWebhook(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	orgExternalID := mux.Vars(r)["orgExternalID"]
	if err := a.userCanAccessOrg(r.Context(), currentUser, orgExternalID); err != nil {
		renderError(w, r, err)
		return
	}

	secretID := mux.Vars(r)["secretID"]
	err := a.db.DeleteOrganizationWebhook(r.Context(), orgExternalID, secretID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
