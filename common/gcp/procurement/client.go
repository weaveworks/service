package procurement

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2/google"

	"github.com/weaveworks/common/http/client"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/common"
)

// EntitlementState denotes the status of a partner entitlement (or account).
type EntitlementState string

const (
	// See https://cloud.google.com/marketplace/docs/partners/commerce-procurement-api/reference/rest/v1/providers.entitlements#EntitlementState

	// ActivationRequested indicates that the entitlement is being
	// created and the backend has sent a notification to the provider
	// for the activation approval.
	ActivationRequested EntitlementState = "ENTITLEMENT_ACTIVATION_REQUESTED"
	// Active indicates that the entitlement is active.
	Active EntitlementState = "ENTITLEMENT_ACTIVE"
	// PendingCancellation indicates that the entitlement was cancelled
	// by the customer.
	PendingCancellation EntitlementState = "ENTITLEMENT_PENDING_CANCELLATION"
	// Cancelled indicates that the entitlement was cancelled.
	Cancelled EntitlementState = "ENTITLEMENT_CANCELLED"
	// PendingPlanChange indicates that the entitlement is currently
	// active, but there is a pending plan change that is requested
	// by the customer.
	PendingPlanChange EntitlementState = "ENTITLEMENT_PENDING_PLAN_CHANGE"
	// PendingPlanChangeApproval indicates that the entitlement is
	// currently active, but there is a plan change request pending
	// provider approval.
	PendingPlanChangeApproval EntitlementState = "ENTITLEMENT_PENDING_PLAN_CHANGE_APPROVAL"
	// Suspended indicates that the entitlement is suspended either
	// by Google or provider request.
	// Note: at time of implementation, Google documentation states this is not yet supported.
	Suspended EntitlementState = "ENTITLEMENT_SUSPENDED"
)

const (
	basePath   = "https://cloudcommerceprocurement.googleapis.com"
	oauthScope = "https://www.googleapis.com/auth/cloud-platform"
)

var clientRequestCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: "google",
	Subsystem: "procurement_client",
	Name:      "request_duration_seconds",
	Help:      "Response time of Google ProcurementAPI requests.",
	Buckets:   instrument.DefBuckets,
})

func init() {
	clientRequestCollector.Register()
}

// API defines methods to interact with the Google Partner Procurement API.
type API interface {
	// ApproveAccount marks the account as approved.
	ApproveAccount(ctx context.Context, externalAccountID string) error

	// ApproveEntitlement marks the entitlement as approved.
	ApproveEntitlement(ctx context.Context, name string) error
	// ApprovePlanChangeEntitlement approves the plan change.
	ApprovePlanChangeEntitlement(ctx context.Context, name, pendingPlanName string) error
	// GetEntitlement fetches the entitlement from the Procurement API.
	GetEntitlement(ctx context.Context, name string) (*Entitlement, error)
	// ListEntitlements returns all entitlements found of given user.
	ListEntitlements(ctx context.Context, externalAccountID string) ([]Entitlement, error)

	// ResourceName builds the name according to the format
	// `providers/<provider-id>/<collection>/<id>`
	ResourceName(collection, id string) string
}

// Client provides access to Google Partner Procurement API
type Client struct {
	*common.JSONClient
	cfg Config
}

// Entitlement represents a procured product of a customer.
// See https://cloud.google.com/marketplace/docs/partners/commerce-procurement-api/reference/rest/v1/providers.entitlements
type Entitlement struct {
	Name             string           `json:"name"`     // "providers/{provider_id}/entitlements/{entitlement_id}"
	Account          string           `json:"account"`  // "providers/{provider_id}/accounts/{account_id}"
	Provider         string           `json:"provider"` // Same as the configured providerid (e.g., "weaveworks-public")
	Product          string           `json:"product"`  // "weave-cloud"
	Plan             string           `json:"plan"`     // "standard"|"enterprise"
	State            EntitlementState `json:"state"`
	NewPendingPlan   string           `json:"newPendingPlan"`
	UpdateTime       time.Time        `json:"updateTime"`
	CreateTime       time.Time        `json:"createTime"`
	UsageReportingID string           `json:"usageReportingId"` // For use in Service Control API when sending usage
	MessageToUser    string           `json:"messageToUser"`
}

// APIError represents an error returned by the API.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
	Details struct {
		Links struct {
			Desc string `json:"description"`
			URL  string `json:"url"`
		} `json:"links"`
	} `json:"details"`
}

// ErrorResponse includes the APIError if a request is unsuccessful.
type ErrorResponse struct {
	Error *APIError `json:"error,omitempty"`
}

// Error returns status code and message.
func (a *APIError) Error() string {
	return fmt.Sprintf("Procurement API request failed: (%d) %s", a.Code, a.Message)
}

// AccountID extracts the account id from the referenced parent account.
func (e Entitlement) AccountID() string {
	s := strings.Split(e.Account, "/")
	if len(s) != 4 {
		return ""
	}
	return s[3]
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

// url builds the URL with the given name and method. Name
// is expected to have the form `providers/<provider-id>/<collection-name>/<item-id>`
// such as `providers/weaveworks-public/entitlements/1234598`.
func (c *Client) url(name, method string) string {
	if method != "" {
		method = ":" + method
	}
	return fmt.Sprintf("%s/v1/%s%s", basePath, name, method)
}

// ResourceName implements API.
func (c *Client) ResourceName(collection, id string) string {
	return fmt.Sprintf("providers/%s/%s/%s", c.cfg.ProviderID, collection, id)
}

// ApproveAccount implements API.
func (c *Client) ApproveAccount(ctx context.Context, externalAccountID string) error {
	var response ErrorResponse
	// This request fails if the body isn't a valid json object
	data := map[string]string{"approvalName": "signup"}
	name := c.ResourceName("accounts", externalAccountID)
	err := c.Post(ctx, "account:approve", c.url(name, "approve"), data, &response)
	if err != nil {
		if response.Error != nil {
			return response.Error
		}
		return err
	}
	return nil
}

// ApproveEntitlement implements API.
func (c *Client) ApproveEntitlement(ctx context.Context, name string) error {
	var response ErrorResponse
	// This request fails if the body isn't a valid json object
	data := map[string]string{}
	err := c.Post(ctx, "entitlement:approve", c.url(name, "approve"), data, &response)
	if err != nil {
		if response.Error != nil {
			return response.Error
		}
		return err
	}
	return nil
}

// ApprovePlanChangeEntitlement implements API.
func (c *Client) ApprovePlanChangeEntitlement(ctx context.Context, name, pendingPlanName string) error {
	data := map[string]string{"pendingPlanName": pendingPlanName}
	var response ErrorResponse
	err := c.Post(ctx, "entitlement:approvePlanChange", c.url(name, "approvePlanChange"), data, &response)
	if err != nil {
		if response.Error != nil {
			return response.Error
		}
		return err
	}
	return nil
}

func isNotFound(err error) bool {
	hse, ok := err.(*common.HTTPStatusError)
	return ok && hse.Code == http.StatusNotFound
}

// GetEntitlement implements API.
func (c *Client) GetEntitlement(ctx context.Context, name string) (*Entitlement, error) {
	var response struct {
		ErrorResponse
		Entitlement
	}
	err := c.Get(ctx, "entitlement:get", c.url(name, ""), &response)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		if response.Error != nil {
			return nil, response.Error
		}
		return nil, err
	}
	return &response.Entitlement, err
}

// ListEntitlements implements API.
func (c *Client) ListEntitlements(ctx context.Context, externalAccountID string) ([]Entitlement, error) {
	var response struct {
		ErrorResponse
		Entitlements []Entitlement `json:"entitlements"`
	}
	q := url.Values{"filter": []string{"account=" + externalAccountID}}
	u := c.url(c.ResourceName("entitlements", ""), "") + "?" + q.Encode()
	err := c.Get(ctx, "entitlement:list", u, &response)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		if response.Error != nil {
			return nil, response.Error
		}
		return nil, err
	}
	return response.Entitlements, nil
}
