package api

import (
	"context"
	"time"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/history"
	"github.com/weaveworks/service/flux-api/service"
)

// UI defines the flux-api methods which are only used for the Weave Cloud UI.
type UI interface {
	Status(ctx context.Context, withPlatform bool) (service.Status, error)
	History(context.Context, update.ResourceSpec, time.Time, int64, time.Time) ([]history.Entry, error)
}

// Upstream defines the flux-api methods which a flux daemon may call.
type Upstream interface {
	RegisterDaemon(context.Context, api.UpstreamServer) error
	LogEvent(context.Context, event.Event) error
}
