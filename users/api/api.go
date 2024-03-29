package api

import (
	"net/http"

	"github.com/gorilla/mux"

	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/gcp/procurement"
	"github.com/weaveworks/service/users"
	users_sync "github.com/weaveworks/service/users-sync/api"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/marketing"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/templates"
)

// API implements the users api.
type API struct {
	createAdminUsers         bool // Whether login without email confirmation is allowed. Development only!
	sessions                 sessions.Store
	db                       db.DB
	logins                   login.Provider
	templates                templates.Engine
	emailer                  emailer.Emailer
	marketingQueues          marketing.Queues
	forceFeatureFlags        []string // Appends flags to every organization that is returned in a GET request.
	marketoMunchkinKey       string
	webhookTokens            map[string]struct{}
	grpc                     users.UsersServer
	procurement              procurement.API
	fluxStatusAPI            string
	scopeProbesAPI           string
	promMetricsAPI           string
	cortexStatsAPI           string
	netPeersAPI              string
	billingClient            billing_grpc.BillingClient
	billingEnabler           featureflag.Enabler
	notificationReceiversURL string
	usersSyncClient          users_sync.UsersSyncClient
	http.Handler
}

// New creates a new API
func New(
	createAdminUsers bool,
	emailer emailer.Emailer,
	sessions sessions.Store,
	db db.DB,
	logins login.Provider,
	templates templates.Engine,
	marketingQueues marketing.Queues,
	forceFeatureFlags []string,
	marketoMunchkinKey string,
	grpc users.UsersServer,
	webhookTokens map[string]struct{},
	procurementClient procurement.API,
	fluxStatusAPI string,
	scopeProbesAPI string,
	promMetricsAPI string,
	cortexStatsAPI string,
	netPeersAPI string,
	billingClient billing_grpc.BillingClient,
	billingEnabler featureflag.Enabler,
	notificationReceiversURL string,
	usersSyncClient users_sync.UsersSyncClient,
) *API {
	a := &API{
		createAdminUsers:         createAdminUsers,
		sessions:                 sessions,
		db:                       db,
		logins:                   logins,
		templates:                templates,
		emailer:                  emailer,
		marketingQueues:          marketingQueues,
		forceFeatureFlags:        forceFeatureFlags,
		marketoMunchkinKey:       marketoMunchkinKey,
		webhookTokens:            webhookTokens,
		grpc:                     grpc,
		procurement:              procurementClient,
		fluxStatusAPI:            fluxStatusAPI,
		scopeProbesAPI:           scopeProbesAPI,
		promMetricsAPI:           promMetricsAPI,
		cortexStatsAPI:           cortexStatsAPI,
		netPeersAPI:              netPeersAPI,
		billingClient:            billingClient,
		billingEnabler:           billingEnabler,
		notificationReceiversURL: notificationReceiversURL,
		usersSyncClient:          usersSyncClient,
	}

	r := mux.NewRouter()
	a.RegisterRoutes(r)
	a.Handler = r
	return a
}
