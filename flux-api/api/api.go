package api

import (
	"context"
	"time"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/config"
	"github.com/weaveworks/service/flux-api/history"
	"github.com/weaveworks/service/flux-api/service"
)

type Service interface {
	api.Client
	api.Upstream

	Status(context.Context) (service.Status, error)
	History(context.Context, update.ResourceSpec, time.Time, int64, time.Time) ([]history.Entry, error)
	GetConfig(ctx context.Context, fingerprint string) (config.Instance, error)
	SetConfig(context.Context, config.Instance) error
	PatchConfig(context.Context, config.Patch) error
}
