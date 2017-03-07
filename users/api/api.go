package api

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/marketing"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/templates"
)

// API implements the users api.
type API struct {
	directLogin        bool
	sessions           sessions.Store
	db                 db.DB
	logins             *login.Providers
	templates          templates.Engine
	emailer            emailer.Emailer
	marketingQueues    marketing.Queues
	forceFeatureFlags  []string
	marketoMunchkinKey string
	grpc               users.UsersServer
	http.Handler
}

// New creates a new API
func New(
	directLogin bool,
	emailer emailer.Emailer,
	sessions sessions.Store,
	db db.DB,
	logins *login.Providers,
	templates templates.Engine,
	marketingQueues marketing.Queues,
	forceFeatureFlags []string,
	marketoMunchkinKey string,
	grpc users.UsersServer,
) *API {
	a := &API{
		directLogin:        directLogin,
		sessions:           sessions,
		db:                 db,
		logins:             logins,
		templates:          templates,
		emailer:            emailer,
		marketingQueues:    marketingQueues,
		forceFeatureFlags:  forceFeatureFlags,
		marketoMunchkinKey: marketoMunchkinKey,
		grpc:               grpc,
	}

	r := mux.NewRouter()
	a.RegisterRoutes(r)
	a.Handler = r
	return a
}
