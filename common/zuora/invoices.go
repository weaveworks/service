package zuora

import (
	"context"
	"encoding/json"
)

const (
	getInvoicesPath   = "transactions/invoices/accounts/%s"
	createInvoicePath = "operations/invoice-collect"

	// Invoice statuses, as found on
	// https://knowledgecenter.zuora.com/DC_Developers/G_SOAP_API/E1_SOAP_API_Object_Reference/Invoice#Status

	// InvoiceStatusPosted refers to an invoice that has been posted.
	InvoiceStatusPosted = "Posted"
	// InvoiceStatusDraft refers to an invoice that has been created but not posted.
	InvoiceStatusDraft = "Draft"
	// InvoiceStatusCanceled refers to a cancelled invoice.
	InvoiceStatusCanceled = "Canceled"
	// InvoiceStatusError refers to an invoice that failed, somehow.
	InvoiceStatusError = "Error"
	// Completed is an import status which Zuora may return upon success.
	Completed = "Completed"
)

// InvoiceItem is an item on an invoice.
type InvoiceItem struct {
	ID                string  `json:"id"`                //: "2c92c0955bfe59cc015c0db4eda631c4",
	SubscriptionName  string  `json:"subscriptionName"`  //: "A-S00000071",
	ServiceStartDate  string  `json:"serviceStartDate"`  //: "2017-03-29",
	ServiceEndDate    string  `json:"serviceEndDate"`    //: "2017-04-28",
	ChargeAmount      float64 `json:"chargeAmount"`      //: 0E-9,
	ChargeDescription string  `json:"chargeDescription"` //: "",
	ChargeName        string  `json:"chargeName"`        //: "Weave Cloud SaaS | Per container second | Usage",
	ProductName       string  `json:"productName"`       //: "Weave Cloud",
	Quantity          float64 `json:"quantity"`          //: 169504608.000000000,
	TaxAmount         float64 `json:"taxAmount"`         //: 0E-9,
	UnitOfMeasure     string  `json:"unitOfMeasure"`     //: "node-seconds",
	ChargeDate        string  `json:"chargeDate"`        //: "2017-05-15 13:03:00",
	ChargeType        string  `json:"chargeType"`        //: "Usage",
	ProcessingType    string  `json:"processingType"`    //: "Charge"
}

// InvoiceFile is a file attached to an invoice.
type InvoiceFile struct {
	ID            string `json:"id"`                   //: "2c92c08a5bf61f48015c0db4ef5548f6",
	VersionNumber int64  `json:"versionNumber"`        //: 1494878580226,
	PdfFileURL    string `json:"pdfFileUrl,omitempty"` //: "https://apisandbox-api.zuora.com/rest/v1/files/2c92c08a5bf61f48015c0db4ef5248f4"
}

// Invoice is a Zuora invoice.
type Invoice struct {
	ID                            string        `json:"id"`                            // "2c92c0955bfe59cc015c0db4eda131be",
	AccountName                   string        `json:"accountName"`                   // "nameless-night-79",
	AccountNumber                 string        `json:"accountNumber"`                 // "Wbee8866756aee4702b5e4f9021a44a2",
	InvoiceDate                   string        `json:"invoiceDate"`                   // "2017-04-15",
	InvoiceNumber                 string        `json:"invoiceNumber"`                 // "INV00000016",
	DueDate                       string        `json:"dueDate"`                       // "2017-04-15",
	InvoiceTargetDate             string        `json:"invoiceTargetDate"`             // "2017-05-15",
	Amount                        float64       `json:"amount"`                        // 0E-9,
	Balance                       float64       `json:"balance"`                       // 0E-9,
	CreditBalanceAdjustmentAmount float64       `json:"creditBalanceAdjustmentAmount"` // 0E-9,
	Status                        string        `json:"status"`                        // "Draft"|"Posted"|"Cancelled"|"Error"
	Body                          string        `json:"body"`                          // "https://apisandbox-api.zuora.com/rest/v1/files/2c92c08a5bf61f48015c0db4ef5248f4",
	InvoiceItems                  []InvoiceItem `json:"invoiceItems"`
	InvoiceFiles                  []InvoiceFile `json:"invoiceFiles"`
}

// invoicesResponse is the response received from zuora
type invoicesResponse struct {
	genericZuoraResponse
	Invoices []Invoice `json:"invoices"`
	NextPage string    `json:"nextPage"`
}

type invoiceResponse struct {
	genericZuoraResponse
	Invoice json.RawMessage
}

type createInvoiceRequest struct {
	AccountKey string `json:"accountKey"`
}

type createInvoiceResponse struct {
	genericZuoraResponse
	PaymentID string `json:"paymentId"` // Used to uniquely indentify the request/response
}

// GetInvoices gets the invoices for a Weave organization.
func (z *Zuora) GetInvoices(ctx context.Context, zuoraAccountNumber, page, pageSize string) ([]Invoice, error) {
	if zuoraAccountNumber == "" {
		return nil, ErrInvalidAccountNumber
	}
	url := z.URL(getInvoicesPath, zuoraAccountNumber)
	url = url + "?" + pagingParams(page, pageSize).Encode()
	resp := &invoicesResponse{}
	if err := z.Get(ctx, getInvoicesPath, url, resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		var err error = resp
		if resp.hasErrorCodeCategory(errorCodeCategoryNotFound) {
			err = ErrNotFound
		}
		return nil, err
	}
	return resp.Invoices, nil
}

// CreateInvoice generates and collects an invoice for unbilled usage. The invoice is created,
// its status set to posted, and the customer is charged.
func (z *Zuora) CreateInvoice(ctx context.Context, zuoraAccountNumber string) (string, error) {
	if zuoraAccountNumber == "" {
		return "", ErrInvalidAccountNumber
	}
	resp := &createInvoiceResponse{}
	err := z.Post(
		ctx,
		createInvoicePath,
		z.URL(createInvoicePath),
		&createInvoiceRequest{AccountKey: zuoraAccountNumber},
		resp,
	)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", resp
	}
	return resp.PaymentID, nil
}
