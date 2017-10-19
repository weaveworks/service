package routes

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/billing-api/render"
	"github.com/weaveworks/service/common/zuora"
)

func (a *API) generateInvoiceMAC(orgID, fileID string) []byte {
	mac := hmac.New(sha256.New, a.HMACSecret)
	mac.Write([]byte(fmt.Sprintf("%s/%s", orgID, fileID)))
	return mac.Sum(nil)
}

func (a *API) checkInvoiceMAC(orgID, fileID string, messageMAC []byte) bool {
	expectedMAC := a.generateInvoiceMAC(orgID, fileID)
	return hmac.Equal(messageMAC, expectedMAC)
}

func (a *API) extractFileID(url string) (string, error) {
	prefix := a.Zuora.URL("files/")
	if strings.HasPrefix(url, prefix) {
		return url[len(prefix):], nil
	}
	return "", fmt.Errorf("Unable to extract File ID from %s", url)
}

// convertFileURL converts a Zuora file URL to a url pointing to our API
func (a *API) convertFileURL(orgID string, zuoraURL string) (string, error) {
	fileID, err := a.extractFileID(zuoraURL)
	if err != nil {
		return "", err
	}
	mac := base64.StdEncoding.EncodeToString(a.generateInvoiceMAC(orgID, fileID))
	fileURL := fmt.Sprintf("/api/billing/%s/invoice_file/%s?mac=%s", orgID, fileID, url.QueryEscape(mac))
	return fileURL, nil
}

// mangleInvoice fixes the file URLs in an invoice. Modifies the invoice in place.
func (a *API) mangleInvoice(orgID string, invoice zuora.Invoice) (zuora.Invoice, error) {
	var err error
	if invoice.Body != "" {
		if invoice.Body, err = a.convertFileURL(orgID, invoice.Body); err != nil {
			return invoice, err
		}
	}
	var files []zuora.InvoiceFile
	for _, file := range invoice.InvoiceFiles {
		if file.PdfFileURL == "" {
			continue
		}
		if file.PdfFileURL, err = a.convertFileURL(orgID, file.PdfFileURL); err != nil {
			return invoice, err
		}
		files = append(files, file)
	}
	invoice.InvoiceFiles = files
	return invoice, nil
}

// GetAccountInvoices gets the invoices for an account.
func (a *API) GetAccountInvoices(w http.ResponseWriter, r *http.Request) {
	logger := logging.With(r.Context())
	orgID := mux.Vars(r)["id"]
	invoices, err := a.Zuora.GetInvoices(
		r.Context(),
		orgID,
		r.FormValue("page"),
		r.FormValue("pageSize"),
	)
	if err == zuora.ErrNotFound {
		invoices = []zuora.Invoice{}
	} else if err != nil {
		logger.Errorf("Problem loading invoices: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	postedInvoices := []zuora.Invoice{}
	for _, invoice := range invoices {
		// See https://knowledgecenter.zuora.com/DC_Developers/C_REST_API/B_REST_API_reference/Transactions/GET_invoices
		// Status can be `Posted`, `Draft`, `Canceled`, `Error`. We only want to show users posted invoices.
		if invoice.Status != zuora.InvoiceStatusPosted {
			continue
		}
		invoice, err = a.mangleInvoice(orgID, invoice)
		if err != nil {
			logger.Errorf("Error mangling invoice: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		postedInvoices = append(postedInvoices, invoice)
	}
	render.JSON(w, http.StatusOK, postedInvoices)
}

// ServeInvoice serves a customer invoice from zuora
// we use a previously-generated MAC to ensure that the user is permitted to
// see the invoice in question
func (a *API) ServeInvoice(w http.ResponseWriter, r *http.Request) {
	orgID := mux.Vars(r)["id"]
	fileID := mux.Vars(r)["file_id"]
	mac := r.URL.Query().Get("mac")

	if fileID == "" {
		http.Error(w, "empty file", http.StatusBadRequest)
		return
	}

	macBytes, err := base64.StdEncoding.DecodeString(mac)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid mac %s", mac), http.StatusBadRequest)
		return
	}

	if !a.checkInvoiceMAC(orgID, fileID, macBytes) {
		http.Error(w, mac, http.StatusUnauthorized)
		return
	}

	a.Zuora.ServeFile(r.Context(), w, fileID)
}
