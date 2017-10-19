package api

import (
	"context"
	"time"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux/config"
	"github.com/weaveworks/service/flux/history"
	"github.com/weaveworks/service/flux/service"
)

type Service interface {
	api.Client
	api.Upstream

	Status(context.Context) (service.Status, error)
	History(context.Context, update.ResourceSpec, time.Time, int64, time.Time) ([]history.Entry, error)
	GetConfig(ctx context.Context, fingerprint string) (config.InstanceConfig, error)
	SetConfig(context.Context, config.InstanceConfig) error
	PatchConfig(context.Context, config.ConfigPatch) error
}
