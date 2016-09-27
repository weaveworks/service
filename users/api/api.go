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
	directLogin       bool
	sessions          sessions.Store
	db                db.DB
	logins            *login.Providers
	templates         templates.Engine
	emailer           emailer.Emailer
	marketingQueues   []*marketing.Queue
	forceFeatureFlags []string
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
	marketingQueues []*marketing.Queue,
	forceFeatureFlags []string,
) *API {
	a := &API{
		directLogin:       directLogin,
		sessions:          sessions,
		db:                db,
		logins:            logins,
		templates:         templates,
		emailer:           emailer,
		marketingQueues:   marketingQueues,
		forceFeatureFlags: forceFeatureFlags,
	}
	a.Handler = a.routes()
	return a
}
