package zuora

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/common/http/client"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/common"
)

var clientRequestCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: common.PrometheusNamespace,
	Subsystem: "zuora_client",
	Name:      "request_duration_seconds",
	Help:      "Response time of zuora requests.",
})

func init() {
	clientRequestCollector.Register()
}

// Client defines an interface to access the Zuora API.
type Client interface {
	GetAuthenticationTokens(ctx context.Context, zuoraAccountNumber string) (*AuthenticationTokens, error)
	GetConfig() Config
	ContainsErrorCode(err interface{}, errorCode int) bool
	NoChargeableUsage(err error) bool
	ChargeableUsageTooLow(err error) bool
	URL(format string, components ...interface{}) string

	GetAccount(ctx context.Context, zuoraAccountNumber string) (*Account, error)
	CreateAccount(ctx context.Context, orgID, currency, firstName, lastName, country, email, state, paymentMethodID string, billCycleDay int, serviceActivationTime time.Time) (*Account, error)
	UpdateAccount(ctx context.Context, zuoraAccountNumber string, userDetails *Account) (*Account, error)
	DeleteAccount(ctx context.Context, zuoraID string) error

	GetInvoices(ctx context.Context, zuoraAccountNumber, page, pageSize string) ([]Invoice, error)
	CreateInvoice(ctx context.Context, zuoraAccountNumber string) (string, error)
	GetCurrentRates(ctx context.Context) (RateMap, error)
	GetPaymentMethod(ctx context.Context, zuoraAccountNumber string) (*CreditCard, error)
	GetPayments(ctx context.Context, zuoraAccountNumber string) ([]*PaymentDetails, error)
	GetPaymentTransactionLog(ctx context.Context, paymentID string) (*PaymentTransaction, error)
	UpdatePaymentMethod(ctx context.Context, paymentMethodID string) error
	UploadUsage(ctx context.Context, r io.Reader) (string, error)
	GetUsage(ctx context.Context, zuoraAccountNumber, page, pageSize string) ([]Usage, error)
	GetUsageImportStatus(ctx context.Context, importID string) (string, error)

	ServeFile(ctx context.Context, w http.ResponseWriter, fileID string)
}

// Zuora implements Client.
type Zuora struct {
	*common.JSONClient
	cfg Config
}

type authClient struct {
	cl   client.Requester
	user string
	pass string
}

func (a authClient) Do(r *http.Request) (*http.Response, error) {
	r.Header.Set("apiAccessKeyId", a.user)
	r.Header.Set("apiSecretAccessKey", a.pass)
	return a.cl.Do(r)
}

type zuoraResponse interface {
	ContainsErrorCode(code int) bool
}

type genericZuoraResponse struct {
	Success bool `json:"success"`
	Reasons []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"reasons,omitempty"`
}

func (r *genericZuoraResponse) Error() string {
	if r.Success {
		// We shouldn't be calling error if this is a success. But this is the
		// closest we can get to a Option type in go. Maybe we should panic here?
		return "success"
	}

	if len(r.Reasons) > 0 {
		if r.hasErrorCodeCategory(errorCodeCategoryNotFound) {
			return ErrNotFound.Error()
		}

		var messages []string
		for _, reason := range r.Reasons {
			messages = append(messages, fmt.Sprintf("%v (error code: %v)", reason.Message, reason.Code))
		}
		return strings.Join(messages, ", ")
	}

	return "unknown error"
}

func (r *genericZuoraResponse) ContainsErrorCode(code int) bool {
	for _, reason := range r.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

const (
	errorCodeCategoryNotFound = 40
)

// hasErrorCodeCategory goes through all response reasons and checks whether it contains
// the given error.
//
// The error code is an 8 digit long number: `5<objectID:5><category:2>`. It tells us
// the response is from the REST API (`5`), the object involved, and what actual error
// category occurred.
// For more info: https://knowledgecenter.zuora.com/DC_Developers/C_REST_API/A_REST_basics/3_Responses_and_errors
func (r *genericZuoraResponse) hasErrorCodeCategory(cat int) bool {
	for _, reason := range r.Reasons {
		if reason.Code%100 == cat {
			return true
		}
	}
	return false
}

// New returns a Zuora. If client is nil, http.Client is instantiated.
func New(cfg Config, httpClient client.Requester) *Zuora {
	if !strings.HasSuffix(cfg.Endpoint, "/") {
		cfg.Endpoint += "/"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.Timeout}
	}
	httpClient = authClient{cl: httpClient, user: cfg.Username, pass: cfg.Password}
	return &Zuora{
		JSONClient: common.NewJSONClient(client.NewTimedClient(httpClient, clientRequestCollector)),
		cfg:        cfg,
	}
}

// GetConfig returns the underlying Config.
func (z *Zuora) GetConfig() Config {
	return z.cfg
}

// ContainsErrorCode casts the provided error into a genericZuoraResponse and returns true if this response contains the provided error code, or false otherwise.
// Indeed, the Zuora API sometimes returns 200/OK despite having actual errors in the response's body, and we expose these as normal Go errors.
// However, in a few occasions, we might want to check the error code returned by Zuora. This method achieves this without revealing Zuora's internals (i.e. genericZuoraResponse).
func (z *Zuora) ContainsErrorCode(err interface{}, errorCode int) bool {
	if resp, ok := err.(zuoraResponse); ok {
		return resp.ContainsErrorCode(errorCode)
	}
	return false
}

// URL on Zuora
func (z *Zuora) URL(format string, components ...interface{}) string {
	return z.cfg.Endpoint + fmt.Sprintf(format, components...)
}

// RestURL on Zuora
func (z *Zuora) RestURL(format string, components ...interface{}) string {
	return z.cfg.RestEndpoint + fmt.Sprintf(format, components...)
}

// pagingParams define the query params zuora accepts for page size and which page
func pagingParams(page, pageSize string) url.Values {
	return url.Values{
		"page":     []string{page},
		"pageSize": []string{pageSize},
	}
}
