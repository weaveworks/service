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
	ApproveAccount(ctx context.Context, externalAccountID string) error

	ApproveEntitlement(ctx context.Context, name, approvalName string) error
	ApprovePlanChangeEntitlement(ctx context.Context, name, pendingPlanName string) error
	GetEntitlement(ctx context.Context, name string) (*Entitlement, error)
	ListEntitlements(ctx context.Context, externalAccountID string) ([]Entitlement, error)
}

// Client provides access to Google Partner Procurement API
type Client struct {
	*common.JSONClient
	cfg Config
}

// Entitlement represents a procured product of a customer.
// See https://cloud.google.com/marketplace/docs/partners/commerce-procurement-api/reference/rest/v1/providers.entitlements
type Entitlement struct {
	Name             string           `json:"name"`     // "entitlements/{entitlement_id}"
	Account          string           `json:"account"`  // "providers/{provider_id}/accounts/{account_id}"
	Provider         string           `json:"provider"` // Same as the configured providerid ("weaveworks-public")
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
	return fmt.Sprintf("request failed: (%d) %s", a.Code, a.Message)
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

func (c *Client) urlEntitlement(name, method string) string {
	if method != "" {
		method = ":" + method
	}
	return fmt.Sprintf("%s/v1/providers/%s/%s%s", basePath, c.cfg.ProviderID, name, method)
}

// ApproveAccount marks the account as approved.
func (c *Client) ApproveAccount(ctx context.Context, externalAccountID string) error {
	var response ErrorResponse
	err := c.Post(ctx, "account:approve",
		fmt.Sprintf("%s/v1/providers/%s/accounts/%s:approve", basePath, c.cfg.ProviderID, externalAccountID),
		nil, &response)
	if err != nil {
		if response.Error != nil {
			return response.Error
		}
		return err
	}
	return nil
}

// ApproveEntitlement marks the entitlement as approved.
// `approvalName` is the source of approval and will be set to
// "signup" by Google if omitted.
func (c *Client) ApproveEntitlement(ctx context.Context, name, approvalName string) error {
	var data map[string]string
	if approvalName != "" {
		data = map[string]string{"approvalName": approvalName}
	}
	var response ErrorResponse
	err := c.Post(ctx, "entitlement:approve", c.urlEntitlement(name, "approve"), data, &response)
	if err != nil {
		if response.Error != nil {
			return response.Error
		}
		return err
	}
	return nil
}

// ApprovePlanChangeEntitlement approves the plan change.
func (c *Client) ApprovePlanChangeEntitlement(ctx context.Context, name, pendingPlanName string) error {
	data := map[string]string{"pendingPlanName": pendingPlanName}
	var response ErrorResponse
	err := c.Post(ctx, "entitlement:approvePlanChange", c.urlEntitlement(name, "approvePlanChange"), data, &response)
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

// GetEntitlement fetches the entitlement from the Procurement API.
func (c *Client) GetEntitlement(ctx context.Context, name string) (*Entitlement, error) {
	var response struct {
		ErrorResponse
		Entitlement Entitlement `json:",inline"`
	}
	err := c.Get(ctx, "entitlement:get", c.urlEntitlement(name, "get"), &response)
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

// ListEntitlements returns all entitlements found of given user.
func (c *Client) ListEntitlements(ctx context.Context, externalAccountID string) ([]Entitlement, error) {
	var response struct {
		ErrorResponse
		Entitlements []Entitlement `json:"entitlements"`
	}
	q := url.Values{"filter": []string{"account=" + externalAccountID}}
	u := c.urlEntitlement("entitlements", "") + "?" + q.Encode()
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
