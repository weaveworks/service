package api

import (
	"context"
	"time"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/config"
	"github.com/weaveworks/service/flux-api/history"
	"github.com/weaveworks/service/flux-api/service"
)

// Service defines the interface for flux-api.
type Service interface {
	api.Client
	api.Upstream

	Status(ctx context.Context, withPlatform bool) (service.Status, error)
	History(context.Context, update.ResourceSpec, time.Time, int64, time.Time) ([]history.Entry, error)
	GetConfig(ctx context.Context, fingerprint string) (config.Instance, error)
	SetConfig(context.Context, config.Instance) error
	PatchConfig(context.Context, config.Patch) error
	// TODO: move to api.Upstream
	ChangeNotify(ctx context.Context, kind string, body interface{}) error
}

// ImageChangeData is ...
// TODO: move to flux repo?
type ImageChangeData struct {
	Repo image.Name
}
