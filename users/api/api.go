package api

import (
	"net/http"

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
	logSuccess         bool
	sessions           sessions.Store
	db                 db.DB
	logins             *login.Providers
	templates          templates.Engine
	emailer            emailer.Emailer
	marketingQueues    marketing.Queues
	forceFeatureFlags  []string
	marketoMunchkinKey string
	http.Handler
}

// New creates a new API
func New(
	directLogin, logSuccess bool,
	emailer emailer.Emailer,
	sessions sessions.Store,
	db db.DB,
	logins *login.Providers,
	templates templates.Engine,
	marketingQueues marketing.Queues,
	forceFeatureFlags []string,
	marketoMunchkinKey string,
) *API {
	a := &API{
		directLogin:        directLogin,
		logSuccess:         logSuccess,
		sessions:           sessions,
		db:                 db,
		logins:             logins,
		templates:          templates,
		emailer:            emailer,
		marketingQueues:    marketingQueues,
		forceFeatureFlags:  forceFeatureFlags,
		marketoMunchkinKey: marketoMunchkinKey,
	}
	a.Handler = a.routes()
	return a
}
