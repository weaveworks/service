package zuora

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/common/http/client"
	"github.com/weaveworks/common/instrument"
)

var clientRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "billing",
	Subsystem: "zuora_client",
	Name:      "request_duration_seconds",
	Help:      "Response time of zuora requests.",
	Buckets:   prometheus.DefBuckets,
}, instrument.HistogramCollectorBuckets)

func init() {
	prometheus.MustRegister(clientRequestDuration)
}

// Client defines an interface to access the Zuora API.
type Client interface {
	GetAuthenticationTokens(ctx context.Context, weaveUserID string) (*AuthenticationTokens, error)
	GetConfig() Config
	ContainsErrorCode(err interface{}, errorCode int) bool
	NoChargeableUsage(err error) bool
	ChargeableUsageTooLow(err error) bool
	URL(format string, components ...interface{}) string

	GetAccount(ctx context.Context, weaveUserID string) (*Account, error)
	CreateAccount(ctx context.Context, orgID, currency, firstName, lastName, country, email, state, paymentMethodID string, billCycleDay int, serviceActivationTime time.Time) (*Account, error)
	UpdateAccount(ctx context.Context, id string, userDetails *Account) (*Account, error)
	DeleteAccount(ctx context.Context, zuoraID string) error

	GetInvoices(ctx context.Context, weaveOrgID, page, pageSize string) ([]Invoice, error)
	CreateInvoice(ctx context.Context, weaveOrgID string) (string, error)
	GetCurrentRates(ctx context.Context) (RateMap, error)
	GetPaymentMethod(ctx context.Context, weaveUserID string) (*CreditCard, error)
	UpdatePaymentMethod(ctx context.Context, paymentMethodID string) error
	UploadUsage(ctx context.Context, r io.Reader) (string, error)
	GetUsage(ctx context.Context, weaveOrgID, page, pageSize string) ([]Usage, error)
	GetUsageImportStatus(ctx context.Context, importID string) (string, error)

	ServeFile(ctx context.Context, w http.ResponseWriter, fileID string)
}

// Zuora implements Client.
type Zuora struct {
	cfg        Config
	httpClient client.Requester
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
func New(cfg Config, httpClient client.Requester) (*Zuora, error) {
	if !strings.HasSuffix(cfg.Endpoint, "/") {
		cfg.Endpoint += "/"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.Timeout}
	}
	return &Zuora{
		cfg:        cfg,
		httpClient: httpClient,
	}, nil
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

// pagingParams define the query params zuora accepts for page size and which page
func pagingParams(page, pageSize string) url.Values {
	return url.Values{
		"page":     []string{page},
		"pageSize": []string{pageSize},
	}
}

func (z *Zuora) do(ctx context.Context, method string, r *http.Request) (*http.Response, error) {
	r.Header.Set("apiAccessKeyId", z.cfg.Username)
	r.Header.Set("apiSecretAccessKey", z.cfg.Password)
	if r.Header.Get("Content-Type") == "" {
		r.Header.Set("Content-Type", "application/json")
	}
	return client.TimeRequestHistogram(ctx, method, clientRequestDuration, z.httpClient, r)
}

func (z *Zuora) get(ctx context.Context, operation, url string) (*http.Response, error) {
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return z.do(ctx, operation, r)
}

func (z *Zuora) head(ctx context.Context, operation, url string) (*http.Response, error) {
	r, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}
	return z.do(ctx, operation, r)
}

func (z *Zuora) send(ctx context.Context, operation, method, url, contentType string, body io.Reader) (*http.Response, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", contentType)
	return z.do(ctx, operation, r)
}

func (z *Zuora) getJSON(ctx context.Context, operation, url string, dest interface{}) error {
	r, err := z.get(ctx, operation, url)
	if err != nil {
		return err
	}
	return z.parseJSON(r, dest)
}

func (z *Zuora) parseJSON(resp *http.Response, dest interface{}) error {
	defer resp.Body.Close()
	// TODO: Handle http status code errors
	return json.NewDecoder(resp.Body).Decode(dest)
}

func (z *Zuora) sendJSON(ctx context.Context, operation, method, url string, data interface{}) (*http.Response, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return z.send(ctx, operation, method, url, "application/json", bytes.NewReader(body))
}

func (z *Zuora) post(ctx context.Context, operation, url, contentType string, body io.Reader) (*http.Response, error) {
	return z.send(ctx, operation, "POST", url, contentType, body)
}

func (z *Zuora) postJSON(ctx context.Context, operation, url string, data interface{}) (*http.Response, error) {
	return z.sendJSON(ctx, operation, "POST", url, data)
}

func (z *Zuora) postForm(ctx context.Context, operation, url string, data url.Values) (*http.Response, error) {
	return z.post(ctx, operation, url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func (z *Zuora) put(ctx context.Context, operation, url, contentType string, body io.Reader) (*http.Response, error) {
	return z.send(ctx, operation, "PUT", url, contentType, body)
}

func (z *Zuora) putJSON(ctx context.Context, operation, url string, data interface{}) (*http.Response, error) {
	return z.sendJSON(ctx, operation, "PUT", url, data)
}
