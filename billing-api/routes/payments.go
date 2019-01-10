package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/weaveworks/service/common/permission"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
)

// GetAuthToken is a HTTP handler to retrieve the auth token.
func (a *API) GetAuthToken(w http.ResponseWriter, r *http.Request) {
	resp, err := a.getOrganization(r.Context(), mux.Vars(r)["id"])
	if err != nil {
		renderError(w, r, err)
		return
	}
	org := resp.Organization
	account, err := a.Zuora.GetAuthenticationTokens(r.Context(), org.ZuoraAccountNumber)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, account)
}

// GetPaymentMethod is a HTTP handler to retrieve an account's payment method.
func (a *API) GetPaymentMethod(w http.ResponseWriter, r *http.Request) {
	resp, err := a.getOrganization(r.Context(), mux.Vars(r)["id"])
	if err != nil {
		renderError(w, r, err)
		return
	}
	org := resp.Organization
	method, err := a.Zuora.GetPaymentMethod(r.Context(), org.ZuoraAccountNumber)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, method)
}

// UpdatePaymentMethod is a HTTP handler to update a payment method.
func (a *API) UpdatePaymentMethod(w http.ResponseWriter, r *http.Request) {
	if _, err := a.Users.RequireOrgMemberPermissionTo(r.Context(), &users.RequireOrgMemberPermissionToRequest{
		UserID:        getRequestUserID(r),
		OrgExternalID: mux.Vars(r)["id"],
		PermissionID:  permission.UpdateBilling,
	}); err != nil {
		renderError(w, r, err)
		return
	}

	if err := a.Zuora.UpdatePaymentMethod(r.Context(), mux.Vars(r)["payment_id"]); err != nil {
		renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
