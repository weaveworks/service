package routes_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing-api/routes"
	"github.com/weaveworks/service/common/zuora"
	"github.com/weaveworks/service/common/zuora/mockzuora"
)

const sampleFileURL = "https://apisandbox-api.zuora.com/rest/v1/files/2c92c08c5e0d9bd5015e0f84943b3ad0"

type zuoraStubInvoices struct {
	mockzuora.StubClient

	err      error
	statuses []string
	noFile   bool
}

func (z *zuoraStubInvoices) GetInvoices(ctx context.Context, weaveOrgID, page, pageSize string) ([]zuora.Invoice, error) {
	if z.err != nil {
		return nil, z.err
	}
	fileURL := sampleFileURL
	if z.noFile {
		fileURL = ""
	}
	if z.statuses == nil {
		return []zuora.Invoice{createInvoice(zuora.InvoiceStatusPosted, fileURL)}, nil
	}

	invoices := []zuora.Invoice{}
	for _, status := range z.statuses {
		invoices = append(invoices, createInvoice(status, fileURL))
	}
	return invoices, nil
}

func (z *zuoraStubInvoices) CreateInvoice(ctx context.Context, weaveOrgID string) (string, error) {
	return "stubPaymentID", nil
}

func createInvoice(status, fileURL string) zuora.Invoice {
	return zuora.Invoice{
		ID:                "2c92c0955e0d9cb9015e0f8492c00ef5",
		AccountName:       "feisty-resonance-96",
		AccountNumber:     "Wbee8866756aee4702b5e4f9021a44a2",
		InvoiceDate:       "2017-08-23",
		InvoiceNumber:     "INV00000030",
		DueDate:           "2017-08-23",
		InvoiceTargetDate: "2017-09-08",
		Amount:            19.86,
		Balance:           19.86,
		CreditBalanceAdjustmentAmount: 0,
		Status: status,
		Body:   fileURL,
		InvoiceItems: []zuora.InvoiceItem{
			{
				ID:                "2c92c0955e0d9cb9015e0f8492c40ef7",
				SubscriptionName:  "A-S00000134",
				ServiceStartDate:  "2017-08-07",
				ServiceEndDate:    "2017-09-06",
				ChargeAmount:      19.86,
				ChargeDescription: "",
				ChargeName:        "Weave Cloud SaaS | Per node second | TEST",
				ProductName:       "Weave Cloud",
				Quantity:          1.739895e+06,
				TaxAmount:         0,
				UnitOfMeasure:     "node-seconds",
				ChargeDate:        "2017-08-23 07:35:00",
				ChargeType:        "Usage",
				ProcessingType:    "Charge",
			},
		},
		InvoiceFiles: []zuora.InvoiceFile{
			{
				ID:            "2c92c08c5e0d9bd5015e0f84943d3ad2",
				VersionNumber: 1503498900236,
				PdfFileURL:    fileURL,
			},
		},
	}
}

func TestGetAccountInvoices(t *testing.T) {
	api := &routes.API{Zuora: &zuoraStubInvoices{}}

	response := doRequest(t, api, http.StatusOK)
	assert.JSONEq(t,
		`[{"id":"2c92c0955e0d9cb9015e0f8492c00ef5","accountName":"feisty-resonance-96","accountNumber":"Wbee8866756aee4702b5e4f9021a44a2","invoiceDate":"2017-08-23","invoiceNumber":"INV00000030","dueDate":"2017-08-23","invoiceTargetDate":"2017-09-08","amount":19.86,"balance":19.86,"creditBalanceAdjustmentAmount":0,"status":"Posted","body":"/api/billing/hello-there-99/invoice_file/files/2c92c08c5e0d9bd5015e0f84943b3ad0?mac=vAkw8Q2pSUcsj35RUJ2LuBTGFsRUOG8SZkEWv1%2BtPAQ%3D","invoiceItems":[{"id":"2c92c0955e0d9cb9015e0f8492c40ef7","subscriptionName":"A-S00000134","serviceStartDate":"2017-08-07","serviceEndDate":"2017-09-06","chargeAmount":19.86,"chargeDescription":"","chargeName":"Weave Cloud SaaS | Per node second | TEST","productName":"Weave Cloud","quantity":1739895,"taxAmount":0,"unitOfMeasure":"node-seconds","chargeDate":"2017-08-23 07:35:00","chargeType":"Usage","processingType":"Charge"}],"invoiceFiles":[{"id":"2c92c08c5e0d9bd5015e0f84943d3ad2","versionNumber":1503498900236,"pdfFileUrl":"/api/billing/hello-there-99/invoice_file/files/2c92c08c5e0d9bd5015e0f84943b3ad0?mac=vAkw8Q2pSUcsj35RUJ2LuBTGFsRUOG8SZkEWv1%2BtPAQ%3D"}]}]`,
		string(response))
}

func TestGetAccountInvoices_PostedOnly(t *testing.T) {
	api := &routes.API{Zuora: &zuoraStubInvoices{statuses: []string{
		zuora.InvoiceStatusPosted,
		zuora.InvoiceStatusCanceled,
		zuora.InvoiceStatusError,
		zuora.InvoiceStatusDraft,
	}}}

	response := doRequest(t, api, http.StatusOK)
	invoices := parseInvoices(t, response)
	assert.Len(t, invoices, 1)
}

func TestGetAccountInvoices_UnknownStatus(t *testing.T) {
	api := &routes.API{Zuora: &zuoraStubInvoices{statuses: []string{
		zuora.InvoiceStatusPosted,
		"youdontknowme",
	}}}

	response := doRequest(t, api, http.StatusOK)
	invoices := parseInvoices(t, response)
	assert.Len(t, invoices, 1)
}

func TestGetAccountInvoices_NotFound(t *testing.T) {
	api := &routes.API{Zuora: &zuoraStubInvoices{err: zuora.ErrNotFound}}

	response := doRequest(t, api, http.StatusOK)
	invoices := parseInvoices(t, response)
	assert.Len(t, invoices, 0)
}

func TestGetAccountInvoices_UnknownError(t *testing.T) {
	api := &routes.API{Zuora: &zuoraStubInvoices{err: errors.New("some unknown error")}}

	doRequest(t, api, http.StatusInternalServerError)
}

func TestGetAccountInvoices_MissingPDF(t *testing.T) {
	api := &routes.API{Zuora: &zuoraStubInvoices{noFile: true}}

	// Missing PDFs should still return the invoice successfully
	response := doRequest(t, api, http.StatusOK)
	invoices := parseInvoices(t, response)
	assert.Len(t, invoices, 1)
	assert.Empty(t, invoices[0].Body)
	assert.Empty(t, invoices[0].InvoiceFiles)
}

func doRequest(t *testing.T, api *routes.API, expectedCode int) []byte {
	r := mux.NewRouter()
	api.RegisterRoutes(r)
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/billing/hello-there-99/invoices", nil)
	r.ServeHTTP(rec, req)
	assert.Equal(t, expectedCode, rec.Code)

	response, err := ioutil.ReadAll(rec.Body)
	assert.NoError(t, err, "failed reading response body")

	return response
}

func parseInvoices(t *testing.T, r []byte) []zuora.Invoice {
	invoices := []zuora.Invoice{}
	err := json.Unmarshal(r, &invoices)
	assert.NoError(t, err, "failed unmarshaling: "+string(r))
	return invoices
}
