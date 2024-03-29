package mockzuora

import (
	"context"
	"flag"
	"io"
	"net/http"
	"time"

	"github.com/weaveworks/service/common/zuora"
)

// Config is a global instance to prevent redefined flag errors when
// registering.
var Config zuora.Config

// StubClient implements zuora.Client
type StubClient struct{}

func init() {
	Config.RegisterFlags(flag.CommandLine)
	// The Sandbox is quite slow these days and exceeds the default 10s ever so slightly
	// for https://apisandbox-api.zuora.com/rest/v1/catalog/products
	Config.Timeout = 25 * time.Second
}

// CreateAccount mocks.
func (z StubClient) CreateAccount(ctx context.Context, orgID string, contact zuora.Contact, currency, paymentMethodID string, billCycleDay int, serviceActivationTime time.Time) (*zuora.Account, error) {
	return nil, nil
}

// DeleteAccount mocks.
func (z StubClient) DeleteAccount(ctx context.Context, zuoraID string) error {
	return nil
}

// UploadUsage mocks.
func (z StubClient) UploadUsage(ctx context.Context, r io.Reader, id string) (zuora.UsageUploadID, error) {
	return "", nil
}

// GetAccount mocks.
func (z StubClient) GetAccount(ctx context.Context, zuoraAccountNumber string) (*zuora.Account, error) {
	return nil, nil
}

// UpdateAccount mocks.
func (z StubClient) UpdateAccount(ctx context.Context, zuoraAccountNumber string, userDetails *zuora.Account) (*zuora.Account, error) {
	return nil, nil
}

// GetCurrentRates mocks.
func (z StubClient) GetCurrentRates(ctx context.Context) (zuora.RateMap, error) {
	return zuora.RateMap{}, nil
}

// GetProductsUnitSet mocks.
func (z *StubClient) GetProductsUnitSet(ctx context.Context, productIDs []string) (map[string]bool, error) {
	return nil, nil
}

// GetInvoices mocks.
func (z StubClient) GetInvoices(ctx context.Context, zuoraAccountNumber, page, pageSize string) ([]zuora.Invoice, error) {
	return nil, nil
}

// CreateInvoice mocks.
func (z StubClient) CreateInvoice(ctx context.Context, zuoraAccountNumber string) (string, error) {
	return "", nil
}

// ServeFile mocks.
func (z StubClient) ServeFile(ctx context.Context, w http.ResponseWriter, fileID string) {
}

// GetAuthenticationTokens mocks.
func (z StubClient) GetAuthenticationTokens(ctx context.Context, zuoraAccountNumber string) (*zuora.AuthenticationTokens, error) {
	return nil, nil
}

// GetPaymentMethod mocks.
func (z StubClient) GetPaymentMethod(ctx context.Context, zuoraAccountNumber string) (*zuora.CreditCard, error) {
	return nil, nil
}

// UpdatePaymentMethod mocks.
func (z StubClient) UpdatePaymentMethod(ctx context.Context, paymentMethodID string) error {
	return nil
}

// GetConfig mocks.
func (z StubClient) GetConfig() zuora.Config {
	return Config
}

// ContainsErrorCode mocks.
func (z StubClient) ContainsErrorCode(err interface{}, errorCode int) bool {
	return false
}

// NoChargeableUsage mocks.
func (z StubClient) NoChargeableUsage(err error) bool {
	return false
}

// ChargeableUsageTooLow mocks.
func (z StubClient) ChargeableUsageTooLow(err error) bool {
	return false
}

// URL mocks.
func (z StubClient) URL(format string, components ...interface{}) string {
	return "https://apisandbox-api.zuora.com/rest/v1/"
}

// GetUsage mocks.
func (z StubClient) GetUsage(ctx context.Context, zuoraAccountNumber, page, pageSize string) ([]zuora.Usage, error) {
	return nil, nil
}

// GetUsageImportStatusURL mocks.
func (z StubClient) GetUsageImportStatusURL(usageUploadID zuora.UsageUploadID) string {
	return string(usageUploadID)
}

// GetUsageImportStatus mocks.
func (z StubClient) GetUsageImportStatus(ctx context.Context, importID string) (zuora.ImportStatusResponse, error) {
	return zuora.ImportStatusResponse{}, nil
}

// GetPayments mocks.
func (z StubClient) GetPayments(ctx context.Context, zuoraAccountNumber string) ([]*zuora.PaymentDetails, error) {
	return nil, nil
}

// GetPaymentTransactionLog mocks.
func (z StubClient) GetPaymentTransactionLog(ctx context.Context, paymentID string) (*zuora.PaymentTransaction, error) {
	return nil, nil
}
