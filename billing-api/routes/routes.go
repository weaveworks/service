package routes

import (
	"net/http"

	"github.com/gorilla/mux"
)

// RegisterRoutes registers the users API HTTP routes to the provided Router.
func (a *API) RegisterRoutes(r *mux.Router) {
	for _, route := range []struct {
		name, method, path string
		handler            http.HandlerFunc
	}{
		// Admin
		{"admin", "GET", "/", a.Admin},
		{"admin_invoice_verify", "GET", "/invoice-verify", a.InvoiceVerify},
		{"admin_invoice_verify", "POST", "/invoice-verify", a.PerformInvoiceVerify},

		// Accounts
		{"api_billing_id_accounts", "POST", "/api/billing/{id}/account", a.CreateAccount},
		{"api_billing_id_accounts", "GET", "/api/billing/{id}/account", a.GetAccount},
		{"api_billing_id_accounts", "PATCH", "/api/billing/{id}/account", a.UpdateAccount},
		{"api_billing_id_accounts_trial", "GET", "/api/billing/{id}/trial", a.GetAccountTrial},
		{"api_billing_id_accounts_status", "GET", "/api/billing/{id}/status", a.GetAccountStatus},

		// Invoices
		{"api_billing_id_accounts_invoices", "GET", "/api/billing/{id}/invoices", a.GetAccountInvoices},
		{"api_billing_id_accounts_invoice_file", "GET", "/api/billing/{id}/invoice_file/{file_id}", a.ServeInvoice},

		// Payments
		{"api_billing_id_payments_authtokens", "GET", "/api/billing/{id}/auth_token", a.GetAuthToken},
		{"api_billing_id_payments_payment", "GET", "/api/billing/{id}/payment", a.GetPaymentMethod},
		{"api_billing_id_payments_payment", "POST", "/api/billing/{id}/payment/{payment_id}", a.UpdatePaymentMethod},

		// Usage
		{"api_billing_id_usage", "GET", "/api/billing/{id}/usage", a.GetUsage},
	} {
		r.Handle(route.path, a.corsHandler(route.handler)).Methods(route.method).Name(route.name)
	}
}

func (a *API) corsHandler(h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", a.CORSAllowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type")

		if r.Method == "OPTIONS" {
			return
		}
		h(w, r)
	})
}
