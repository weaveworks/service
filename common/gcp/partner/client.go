package partner

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/weaveworks/common/http/client"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/common"
)

// SubscriptionStatus denotes the status of a partner subscription
type SubscriptionStatus string

const (
	// Pending means the subscription is awaiting approval.
	Pending SubscriptionStatus = "PENDING"
	// Active is a subscription that is running.
	Active SubscriptionStatus = "ACTIVE"
	// Complete are subscriptions that are no longer active (i.e., canceled)
	Complete SubscriptionStatus = "COMPLETE"

	ssoLoginKeyName = "keyForSSOLogin"

	// ServiceLevelLabelKey is the label suffix on the subscribed resource
	ServiceLevelLabelKey = "ServiceLevel"
	// ConsumerIDLabelKey is the label suffix on the subscribed resource
	ConsumerIDLabelKey = "consumerId"
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

// API defines methods to interact with the Google Partner Subscriptions API.
type API interface {
	ApproveSubscription(ctx context.Context, name string, body *RequestBody) (*Subscription, error)
	DenySubscription(ctx context.Context, name string, body *RequestBody) (*Subscription, error)
	GetSubscription(ctx context.Context, name string) (*Subscription, error)
	ListSubscriptions(ctx context.Context, externalAccountID string) ([]Subscription, error)
}

// Client provides access to Google Partner Subscriptions API
type Client struct {
	*common.JSONClient
	cfg Config
}

// Subscription is a plan of a customer.
// See https://cloud.google.com/billing-subscriptions/reference/rest/v1/partnerSubscriptions#PartnerSubscription
type Subscription struct {
	Name                string               `json:"name"` // "partnerSubscriptions/*"
	ExternalAccountID   string               `json:"externalAccountId"`
	Version             string               `json:"version"`
	Status              SubscriptionStatus   `json:"status"`
	SubscribedResources []SubscribedResource `json:"subscribedResources"`
	RequiredApproval    []struct {
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

// SubscribedResource describes a product part of this subscription.
type SubscribedResource struct {
	Labels               map[string]string `json:"labels"`
	Resource             string            `json:"resource"`
	SubscriptionProvider string            `json:"subscriptionProvider"`
}

// ExtractResourceLabel returns the value of key under given resource. It also attempts to
// prefix the key with the Subscription Provider if it can't find it by its own.
//
// Example resource:
// {
// 		"subscriptionProvider": "weaveworks-public-cloudmarketplacepartner.googleapis.com",
// 		"resource": "weave-cloud",
// 		"labels": {
//			"consumerId":"project_number:347113080196",
//			"serviceName":"staging.google.weave.works",
// 			"weaveworks-public-cloudmarketplacepartner.googleapis.com/ServiceLevel": "standard"
// 		}
// 	}
func (s Subscription) ExtractResourceLabel(resource, key string) string {
	for _, res := range s.SubscribedResources {
		if res.Resource == resource {
			value, ok := res.Labels[key]
			if !ok {
				value = res.Labels[fmt.Sprintf("%s/%s", res.SubscriptionProvider, key)]
			}
			return value
		}
	}
	return ""
}

// RequestBodyWithSSOLoginKey builds a request body for the ApproveSubscription()
// calls. It embeds a so-called key which will be sent to us during SSO.
func RequestBodyWithSSOLoginKey(externalAccountID string) *RequestBody {
	return &RequestBody{
		ApprovalID: "default-approval",
		Labels: map[string]string{
			// The value passed here will be sent to us during SSO. It is supposed to
			// allow us to determine which instances the user is supposed to have access to.
			ssoLoginKeyName: externalAccountID,
		},
	}
}

type subscriptionResponse struct {
	Subscription
	Error *errorResponse `json:"error,omitempty"`
}

type listSubscriptionResponse struct {
	Subscriptions []Subscription `json:"subscriptions"`
	Error         *errorResponse `json:"error,omitempty"`
}

// RequestBody contains the fields for sending approvals and denies.
type RequestBody struct {
	ApprovalID   string            `json:"approvalId"`
	ApprovalNote string            `json:"approvalNote"`
	Labels       map[string]string `json:"labels"`
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
//
// Requires one of the following OAuth scopes:
// - https://www.googleapis.com/auth/cloud-platform
// - https://www.googleapis.com/auth/cloud-billing
func NewClient(cfg Config) (*Client, error) {
	jsonKey, err := ioutil.ReadFile(cfg.ServiceAccountKeyFile)
	if err != nil {
		return nil, err
	}
	return NewClientFromJSONKey(cfg, jsonKey)
}

// NewClientFromJSONKey instantiates a client from the given JSON key.
func NewClientFromJSONKey(cfg Config, jsonKey []byte) (*Client, error) {
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

// NewClientFromTokenSource instantiates a client from the given token source.
func NewClientFromTokenSource(ts oauth2.TokenSource) (*Client, error) {
	cl := oauth2.NewClient(context.Background(), ts)
	return &Client{
		JSONClient: common.NewJSONClient(client.NewTimedClient(cl, clientRequestCollector)),
	}, nil
}

// ApproveSubscription marks the subscription approved.
// See https://cloud.google.com/billing-subscriptions/reference/rest/v1/partnerSubscriptions/approve
func (c *Client) ApproveSubscription(ctx context.Context, name string, body *RequestBody) (*Subscription, error) {
	u := fmt.Sprintf("%s/v1/%s:approve", basePath, name)
	resp := &subscriptionResponse{}
	err := c.Post(ctx, "partnerSubscriptions:approve", u, body, resp)
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
func (c *Client) DenySubscription(ctx context.Context, name string, body *RequestBody) (*Subscription, error) {
	u := fmt.Sprintf("%s/v1/%s:deny", basePath, name)
	resp := &subscriptionResponse{}
	err := c.Post(ctx, "partnerSubscriptions:deny", u, body, resp)
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
