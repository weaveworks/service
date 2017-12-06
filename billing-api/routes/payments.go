package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/weaveworks/service/common/render"
)

// GetAuthToken is a HTTP handler to retrieve the auth token.
func (a *API) GetAuthToken(w http.ResponseWriter, r *http.Request) {
	account, err := a.Zuora.GetAuthenticationTokens(r.Context(), mux.Vars(r)["id"])
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, account)
}

// GetPaymentMethod is a HTTP handler to retrieve an account's payment method.
func (a *API) GetPaymentMethod(w http.ResponseWriter, r *http.Request) {
	method, err := a.Zuora.GetPaymentMethod(r.Context(), mux.Vars(r)["id"])
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, method)
}

// UpdatePaymentMethod is a HTTP handler to update a payment method.
func (a *API) UpdatePaymentMethod(w http.ResponseWriter, r *http.Request) {
	if err := a.Zuora.UpdatePaymentMethod(r.Context(), mux.Vars(r)["payment_id"]); err != nil {
		renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
