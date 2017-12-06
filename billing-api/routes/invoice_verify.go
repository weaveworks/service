package routes

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/common/constants/billing"
	"github.com/weaveworks/service/common/render"
	timeutil "github.com/weaveworks/service/common/time"
	"github.com/weaveworks/service/common/zuora"
	"github.com/weaveworks/service/users"
)

const (
	zuoraDateFormat     = "2006-01-02"
	zuoraDatetimeFormat = "2006-01-02 15:04:05"

	csrfTokenPlaceholder = "$__CSRF_TOKEN_PLACEHOLDER__"
)

var pageTemplate = `
<html>
	<head><title>Invoice Verifier</title></head>
	<body>
		<h1>Invoice Verifier</h1>
		<ul>
			<li>
				<form action="invoice-verify" method="post">
				Zuora's Instance Id
				<input type="text" name="instance_id" value="{{.InstanceID}}" />
				<input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
				<button type="submit">Trigger verification</button>
				{{.Result}}
				</form>
			</li>
		</ul>
	</body>
</html>
`

// InvoiceVerify renders the form to enter an invoice id.
func (a *API) InvoiceVerify(w http.ResponseWriter, r *http.Request) {
	renderPage(w, http.StatusOK, "", "", "")
}

// PerformInvoiceVerify kicks off the verification process and renders the results as HTML.
func (a *API) PerformInvoiceVerify(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	instanceID := r.Form.Get("instance_id")
	csrfToken := r.Form.Get("csrf_token")
	if instanceID == "" {
		renderPage(w, http.StatusBadRequest, instanceID, "Instance id is required", csrfToken)
		return
	}
	instanceID = strings.TrimSpace(instanceID)
	err := assertInvoiceVerified(r.Context(), a, instanceID, 3) // checks last three invoices
	if err != nil {
		renderPage(w, http.StatusOK, instanceID, err.Error(), csrfToken)
		return
	}
	renderPage(w, http.StatusOK, instanceID, "Valid!", csrfToken)
}

func renderPage(w http.ResponseWriter, status int, instanceID, result, csrfToken string) {
	if csrfToken == "" {
		csrfToken = csrfTokenPlaceholder
	}
	template := template.Must(template.New("invoice-verification").Parse(pageTemplate))
	// csrf_token has to be passed in because authfe will not substitute
	// $__CSRF_TOKEN_PLACEHOLDER__ on responses generated from POST requests (only GET responses)
	render.HTMLTemplate(w, status, template, map[string]interface{}{
		"InstanceID": instanceID,
		"Result":     result,
		"CSRFToken":  csrfToken,
	})
}

//  assertInvoiceVerified checks if invoice quantities and charge amounts match the same values in usage records otherwise it returns an error
func assertInvoiceVerified(ctx context.Context, a *API, weaveOrgID string, numInvoices int) error {
	logger := logging.With(ctx)

	invoices, usages, err := getInvoicesAndCorrespondingUsage(ctx, a.Zuora, weaveOrgID, string(numInvoices))
	if err != nil {
		return err
	}

	resp, err := a.Users.GetOrganization(ctx, &users.GetOrganizationRequest{
		ID: &users.GetOrganizationRequest_ExternalID{ExternalID: weaveOrgID},
	})
	if err != nil {
		return fmt.Errorf("Failed to fetch organization: %v", err)
	}

	rates, err := a.Zuora.GetCurrentRates(ctx)
	if err != nil {
		return err
	}
	price := rates[billing.UsageNodeSeconds]

	for _, invoice := range invoices {
		invoiceStatus := invoice.Status
		// no point checking for other invoice states
		if invoiceStatus != "Posted" && invoiceStatus != "Draft" {
			continue
		}
		for _, invoiceItem := range invoice.InvoiceItems {
			var invoiceItemQuantity float64
			startTs, err := time.Parse(zuoraDateFormat, invoiceItem.ServiceStartDate)
			if err != nil {
				return err
			}
			endTs, err := time.Parse(zuoraDateFormat, invoiceItem.ServiceEndDate)
			if err != nil {
				return err
			}
			endTs = timeutil.BeginningOfNextDay(endTs) // serviceEndDate is inclusive (zuora)

			for _, u := range usages {
				usageStartTs, err := time.Parse(zuoraDatetimeFormat, u.StartDate)
				if err != nil {
					return err
				}

				if invoiceStatus == "Posted" && u.Status != "Processed" {
					continue
				}
				if invoiceStatus == "Draft" && u.Status != "Processed" {
					continue
				}
				if u.UnitType != billing.UsageNodeSeconds {
					continue
				}
				if timeutil.InTimeRange(startTs, endTs, usageStartTs) {
					invoiceItemQuantity += u.Quantity
				}
			}

			// verify quantities
			logger.Infof("Invoice %v, quantity: %v, usage %v", invoice.InvoiceNumber, invoiceItem.Quantity, invoiceItemQuantity)
			if invoiceItemQuantity != invoiceItem.Quantity {
				return fmt.Errorf("Invoice %v, quantities do not match usage %v != %v", invoice.InvoiceNumber, invoiceItem.Quantity, invoiceItemQuantity)
			}
			// verify charge
			charge := roundHalfUp(price * invoiceItemQuantity)
			logger.Infof("Invoice %v, charge: %v, usage charge %v", invoice.InvoiceNumber, invoiceItem.ChargeAmount, charge)
			if !floatEqual(invoiceItem.ChargeAmount, charge) {
				return fmt.Errorf("Invoice %v, charge amount does not match usage %v != %v", invoice.InvoiceNumber, invoiceItem.ChargeAmount, charge)
			}

			// verify usage data in zuora against ours
			aggs, err := a.DB.GetAggregates(ctx, resp.Organization.ID, startTs, endTs)
			if err != nil {
				return err
			}
			localQuantity := float64(sumAggregates(aggs))
			logger.Infof("Local usage %v, [%v,%v)", localQuantity, startTs, endTs)
			if localQuantity != invoiceItemQuantity {
				return fmt.Errorf("Zuora usage does not match local usage %v != %v", invoiceItemQuantity, localQuantity)
			}
		}
	}
	return nil
}

func getInvoicesAndCorrespondingUsage(ctx context.Context, z zuora.Client, weaveOrgID, pageSize string) ([]zuora.Invoice, []zuora.Usage, error) {
	var invoices []zuora.Invoice
	var usages []zuora.Usage

	invoices, err := z.GetInvoices(ctx, weaveOrgID, "1", pageSize)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to fetch invoices: %v", err)
	}

	// I have invoices, now I need to fetch all usage records that correspond to those invoices:
	// 1. find serviceStartDate of earliest invoice
	// 2. fetch usages until there's one usage record which is earlier than the earliest invoice serviceStartDate
	// (since zuora guarantees that usage records are returned in reverse chronological order)
	firstInvoiceDate, err := minInvoicesDate(invoices)
	if err != nil {
		return nil, nil, err
	}

	for usagePage := 1; ; usagePage++ {
		u, err := z.GetUsage(ctx, weaveOrgID, strconv.Itoa(usagePage), "40") // 40 is the maximum (zuora)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to fetch usage: %v", err)
		}
		// nothing is returned, use what we have
		if len(u) == 0 {
			break
		}
		usages = append(usages, u...)
		lastUsage := u[len(u)-1]
		usageTs, err := time.Parse(zuoraDatetimeFormat, lastUsage.StartDate)
		if err != nil {
			return nil, nil, err
		}
		if usageTs.Before(firstInvoiceDate) {
			break
		}
	}
	return invoices, usages, nil
}

func minInvoicesDate(invoices []zuora.Invoice) (time.Time, error) {
	var min time.Time
	for _, invoice := range invoices {
		for _, invoiceItem := range invoice.InvoiceItems {
			date, err := time.Parse(zuoraDateFormat, invoiceItem.ServiceStartDate)
			if err != nil {
				return min, err
			}
			if min.IsZero() {
				min = date
				continue
			}
			if date.Before(min) {
				min = date
			}
		}
	}
	return min, nil
}

// floatEqual compares floating point numbers, comparing floats is difficult
func floatEqual(lhs, rhs float64) bool {
	const TOLERANCE = 0.000001
	diff := math.Abs(lhs - rhs)
	return diff < TOLERANCE
}

func roundHalfUp(amount float64) float64 {
	// zuora rounds half up by default
	const unit = 0.01
	return float64(int64(amount/unit+0.5)) * unit
}

func sumAggregates(aggs []db.Aggregate) int64 {
	var sum int64
	for _, agg := range aggs {
		if agg.AmountType == billing.UsageNodeSeconds {
			sum += agg.AmountValue
		}
	}
	return sum
}
