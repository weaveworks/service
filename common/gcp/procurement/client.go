package procurement

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"net/http"

	"github.com/weaveworks/common/http/client"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/common"
)

// EntitlementState denotes the status of a partner entitlement (or account).
type EntitlementState string

const (
	// See https://cloud.google.com/marketplace/docs/partners/commerce-procurement-api/reference/rest/v1/providers.entitlements#EntitlementState
	ActivationRequested       EntitlementState = "ENTITLEMENT_ACTIVATION_REQUESTED"
	Active                    EntitlementState = "ENTITLEMENT_ACTIVE"
	PendingCancellation       EntitlementState = "ENTITLEMENT_PENDING_CANCELLATION"
	Cancelled                 EntitlementState = "ENTITLEMENT_CANCELLED"
	PendingPlanChange         EntitlementState = "ENTITLEMENT_PENDING_PLAN_CHANGE"
	PendingPlanChangeApproval EntitlementState = "ENTITLEMENT_PENDING_PLAN_CHANGE_APPROVAL"
	Suspended                 EntitlementState = "ENTITLEMENT_SUSPENDED"
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
	ApproveEntitlement(ctx context.Context, name string) error
	ApprovePlanChangeEntitlement(ctx context.Context, name, pendingPlanName string) error
	GetEntitlement(ctx context.Context, name string) (*Entitlement, error)
	ListEntitlements(ctx context.Context, externalAccountID string) ([]Entitlement, error)
}

// Client provides access to Google Partner Procurement API
type Client struct {
	*common.JSONClient
	cfg Config
}

// See https://cloud.google.com/marketplace/docs/partners/commerce-procurement-api/reference/rest/v1/providers.accounts#Account
type Account struct {
}

// See https://cloud.google.com/marketplace/docs/partners/commerce-procurement-api/reference/rest/v1/providers.entitlements
type Entitlement struct {
	Name             string           `json:"name"`     // "entitlements/{entitlement_id}"
	Account          string           `json:"account"`  // "accounts/{account_id}"
	Provider         string           `json:"provider"` // "weaveworks" FIXME(rndstr): confirm
	Product          string           `json:"product"`  // "weave-cloud" FIXME(rndstr): confirm
	Plan             string           `json:"plan"`     // "standard"|"enterprise" FIXME(rndstr): confirm
	State            EntitlementState `json:"state"`
	NewPendingPlan   string           `json:"newPendingPlan"`
	UpdateTime       time.Time        `json:"updateTime"`
	CreateTime       time.Time        `json:"createTime"`
	UsageReportingID string           `json:"usageReportingId"` // For use in Service Control API when sending usage
	MessageToUser    string           `json:"messageToUser"`
}

func (e Entitlement) AccountID() string {
	s := strings.Split(e.Account, "/")
	if len(s) != 2 {
		return ""
	}
	return s[1]
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
// FIXME(rndstr): do we need this? is this correct?
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
// FIXME(rndstr): do we need this?
func NewClientFromTokenSource(ts oauth2.TokenSource) (*Client, error) {
	cl := oauth2.NewClient(context.Background(), ts)
	return &Client{
		JSONClient: common.NewJSONClient(client.NewTimedClient(cl, clientRequestCollector)),
	}, nil
}

func (c *Client) urlEntitlement(name, method string) string {
	if method != "" {
		method = ":" + method
	}
	return fmt.Sprintf("%s/v1/providers/%s/%s%s", basePath, c.cfg.ProviderID, name, method)
}

// ApproveEntitlement marks the entitlement as approved.
// `apporvalName` seems to be the source of approval, such
// as "signup".
func (c *Client) ApproveEntitlement(ctx context.Context, name string) error {
	// TODO(rndstr): we used to store the ssoLoginKeyName here alongside the subscription.
	// But the procurement API technically no longer supports it and it needs to be seen
	// whether it is actually needed since we no longer have sso login. There is a deprecated
	// field `properties` that could technically be used.
	return c.Post(ctx, "entitlement:approve", c.urlEntitlement(name, "approve"), nil, nil)
}

func (c *Client) ApprovePlanChangeEntitlement(ctx context.Context, name, pendingPlanName string) error {
	data := map[string]string{"pendingPlanName": pendingPlanName}
	return c.Post(ctx, "entitlement:approvePlanChange", c.urlEntitlement(name, "approvePlanChange"), data, nil)
}

func isNotFound(err error) bool {
	hse, ok := err.(*common.HTTPStatusError)
	return ok && hse.Code == http.StatusNotFound
}

func (c *Client) GetEntitlement(ctx context.Context, name string) (*Entitlement, error) {
	var e Entitlement
	err := c.Get(ctx, "entitlement:get", c.urlEntitlement(name, "get"), &e)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &e, err
}

func (c *Client) ListEntitlements(ctx context.Context, externalAccountID string) ([]Entitlement, error) {
	var response struct {
		Entitlements []Entitlement `json:"entitlements"`
	}
	q := url.Values{"filter": []string{"account=" + externalAccountID}}
	err := c.Get(ctx, "entitlement:list", c.urlEntitlement("entitlements", ""), &response)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return response.Entitlements, err
}
