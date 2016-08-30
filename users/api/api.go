package api

import (
	"net/http"

	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/pardot"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/storage"
	"github.com/weaveworks/service/users/templates"
)

// API implements the users api.
type API struct {
	directLogin       bool
	sessions          sessions.Store
	db                storage.Database
	logins            *login.Providers
	templates         templates.Engine
	emailer           emailer.Emailer
	pardotClient      *pardot.Client
	forceFeatureFlags []string
	http.Handler
}

// New creates a new API
func New(
	directLogin bool,
	emailer emailer.Emailer,
	sessions sessions.Store,
	db storage.Database,
	logins *login.Providers,
	templates templates.Engine,
	pardotClient *pardot.Client,
	forceFeatureFlags []string,
) *API {
	a := &API{
		directLogin:       directLogin,
		sessions:          sessions,
		db:                db,
		logins:            logins,
		templates:         templates,
		emailer:           emailer,
		pardotClient:      pardotClient,
		forceFeatureFlags: forceFeatureFlags,
	}
	a.Handler = a.routes()
	return a
}
