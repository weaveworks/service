package mockzuora

import (
	"context"
	"flag"
	"io"
	"net/http"
	"time"

	"github.com/weaveworks/service/billing/zuora"
)

// Config is a global instance to prevent redefined flag errors when
// registering.
var Config zuora.Config

// StubClient implements zuora.Client
type StubClient struct{}

func init() {
	Config.RegisterFlags(flag.CommandLine)
}

// CreateAccount mocks.
func (z StubClient) CreateAccount(ctx context.Context, orgID, currency, firstName, lastName, country, email, state, paymentMethodID string, billCycleDay int, serviceActivationTime time.Time) (*zuora.Account, error) {
	return nil, nil
}

// DeleteAccount mocks.
func (z StubClient) DeleteAccount(ctx context.Context, zuoraID string) error {
	return nil
}

// UploadUsage mocks.
func (z StubClient) UploadUsage(ctx context.Context, r io.Reader) (string, error) {
	return "", nil
}

// GetAccount mocks.
func (z StubClient) GetAccount(ctx context.Context, weaveUserID string) (*zuora.Account, error) {
	return nil, nil
}

// UpdateAccount mocks.
func (z StubClient) UpdateAccount(ctx context.Context, id string, userDetails *zuora.Account) (*zuora.Account, error) {
	return nil, nil
}

// GetCurrentRates mocks.
func (z StubClient) GetCurrentRates(ctx context.Context) (zuora.RateMap, error) {
	return zuora.RateMap{}, nil
}

// GetInvoices mocks.
func (z StubClient) GetInvoices(ctx context.Context, weaveOrgID, page, pageSize string) ([]zuora.Invoice, error) {
	return nil, nil
}

// CreateInvoice mocks.
func (z StubClient) CreateInvoice(ctx context.Context, weaveOrgID string) (string, error) {
	return "", nil
}

// ServeFile mocks.
func (z StubClient) ServeFile(ctx context.Context, w http.ResponseWriter, fileID string) {
}

// GetAuthenticationTokens mocks.
func (z StubClient) GetAuthenticationTokens(ctx context.Context, weaveUserID string) (*zuora.AuthenticationTokens, error) {
	return nil, nil
}

// GetPaymentMethod mocks.
func (z StubClient) GetPaymentMethod(ctx context.Context, weaveUserID string) (*zuora.CreditCard, error) {
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
func (z StubClient) GetUsage(ctx context.Context, weaveOrgID, page, pageSize string) ([]zuora.Usage, error) {
	return nil, nil
}

// GetUsageImportStatus mocks.
func (z StubClient) GetUsageImportStatus(ctx context.Context, importID string) (string, error) {
	return "", nil
}
