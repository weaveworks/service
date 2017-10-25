package partner

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2/google"

	"github.com/weaveworks/common/http/client"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/common"
)

// Status of a subscription.
const (
	StatusUnknown  = "UNKNOWN_STATUS"
	StatusActive   = "ACTIVE"
	StatusComplete = "COMPLETE"
	StatusPending  = "PENDING"
	StatusCanceled = "CANCELED"
)

const (
	basePath   = "https://cloudbilling.googleapis.com"
	oauthScope = "https://www.googleapis.com/auth/cloud-platform"
)

var clientRequestCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: "google",
	Subsystem: "partnersubscriptions_client",
	Name:      "request_duration_seconds",
	Help:      "Response time of Google Partner Subscriptions API requests.",
	Buckets:   prometheus.DefBuckets,
})

func init() {
	clientRequestCollector.Register()
}

// Client provides access to Google Partner Subscriptions API
type Client struct {
	*common.JSONClient
	cfg Config
}

// Subscription is a plan of a customer.
// See https://cloud.google.com/billing-subscriptions/reference/rest/v1/partnerSubscriptions#PartnerSubscription
type Subscription struct {
	Name                string `json:"name"` // "partnerSubscriptions/*"
	ExternalAccountID   string `json:"externalAccountId"`
	Version             string `json:"version"`
	Status              string `json:"status"`
	SubscribedResources []struct {
		Labels               map[string]string `json:"labels"`
		Resource             string            `json:"resource"`
		SubscriptionProvider string            `json:"subscriptionProvider"`
	} `json:"subscribedResources"`
	RequiredApproval []struct {
		Name         string     `json:"name"`
		Status       string     `json:"status"`
		ApprovalTime *time.Time `json:"approvalTime,omitempty"`
		ApprovalNote string     `json:"approvalNote,omitempty"`
	} `json:"requiredApprovals"`

	StartDate  *date     `json:"startDate,omitempty"`
	EndDate    *date     `json:"endDate,omitempty"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
}

type subscriptionResponse struct {
	Subscription
	Error *errorResponse `json:"error,omitempty"`
}

type listSubscriptionResponse struct {
	Subscriptions []Subscription `json:"subscriptions"`
	Error         *errorResponse `json:"error,omitempty"`
}

type errorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// Error includes status, code, and message of the response.
func (e errorResponse) Error() string {
	return fmt.Sprintf("%v (%v): %v", e.Code, e.Status, e.Message)
}

// date represents year/month/day. It knows nothing about time or location.
type date struct {
	Year  int        `json:"year"`
	Month time.Month `json:"month"`
	Day   int        `json:"day"`
}

// Time returns a time.Time representation of this date.
func (d date) Time(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// NewClient returns a Client accessing the Partner Subscriptions API. It uses
// oauth2 for authentication.
func NewClient(cfg Config) (*Client, error) {
	jsonKey, err := ioutil.ReadFile(cfg.ServiceAccountKeyFile)
	if err != nil {
		return nil, err
	}

	// Create oauth2 HTTP client from the given service account key JSON
	jwtConf, err := google.JWTConfigFromJSON(jsonKey, oauthScope)
	if err != nil {
		return nil, err
	}
	cl := jwtConf.Client(context.Background())
	cl.Timeout = cfg.Timeout

	return &Client{
		JSONClient: common.NewJSONClient(client.NewTimedClient(cl, clientRequestCollector)),
		cfg:        cfg,
	}, nil
}

// ApproveSubscription marks the subscription approved.
// See https://cloud.google.com/billing-subscriptions/reference/rest/v1/partnerSubscriptions/approve
func (c *Client) ApproveSubscription(ctx context.Context, name string) (*Subscription, error) {
	u := fmt.Sprintf("%s/v1/%s:approve", basePath, name)
	resp := &subscriptionResponse{}
	err := c.Post(ctx, "partnerSubscriptions:approve", u, nil, resp)
	if resp.Error != nil {
		return nil, resp.Error
	}
	if err != nil {
		return nil, err
	}

	return &resp.Subscription, nil
}

// DenySubscription marks the subscription denied.
// See https://cloud.google.com/billing-subscriptions/reference/rest/v1/partnerSubscriptions/deny
func (c *Client) DenySubscription(ctx context.Context, name string) (*Subscription, error) {
	u := fmt.Sprintf("%s/v1/%s:deny", basePath, name)
	resp := &subscriptionResponse{}
	err := c.Post(ctx, "partnerSubscriptions:deny", u, nil, resp)
	if resp.Error != nil {
		return nil, resp.Error
	}
	if err != nil {
		return nil, err
	}

	return &resp.Subscription, nil
}

// GetSubscription returns the requested subscription.
// See https://cloud.google.com/billing-subscriptions/reference/rest/v1/partnerSubscriptions/get
func (c *Client) GetSubscription(ctx context.Context, name string) (*Subscription, error) {
	u := fmt.Sprintf("%s/v1/%s", basePath, name)
	resp := &subscriptionResponse{}
	err := c.Get(ctx, "partnerSubscriptions:get", u, resp)
	if resp.Error != nil {
		return nil, resp.Error
	}
	if err != nil {
		return nil, err
	}

	return &resp.Subscription, nil
}

// ListSubscriptions returns all subscriptions for the given account.
// See https://cloud.google.com/billing-subscriptions/reference/rest/v1/partnerSubscriptions/list
func (c *Client) ListSubscriptions(ctx context.Context, externalAccountID string) ([]Subscription, error) {
	q := url.Values{"externalAccountId": []string{externalAccountID}}
	u := fmt.Sprintf("%s/v1/partnerSubscriptions?%s", basePath, q.Encode())
	resp := &listSubscriptionResponse{}
	err := c.Get(ctx, "partnerSubscriptions:list", u, resp)
	if resp.Error != nil {
		return nil, resp.Error
	}
	if err != nil {
		return nil, err
	}

	return resp.Subscriptions, nil
}
