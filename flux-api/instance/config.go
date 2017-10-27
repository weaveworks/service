package instance

import (
	"time"

	"github.com/weaveworks/service/flux-api/config"
	"github.com/weaveworks/service/flux-api/service"
)

// Connection contains information about an instance connection.
type Connection struct {
	Last      time.Time `json:"last"`
	Connected bool      `json:"connected"`
}

// Config contains information about an instance configuration.
type Config struct {
	Settings   config.Instance `json:"settings"`
	Connection Connection      `json:"connection"`
}

// UpdateFunc takes a Config and returns another Config.
type UpdateFunc func(config Config) (Config, error)

// DB is the instance DB interface.
type DB interface {
	UpdateConfig(instance service.InstanceID, update UpdateFunc) error
	GetConfig(instance service.InstanceID) (Config, error)
}

type configurer struct {
	instance service.InstanceID
	db       DB
}

func (c configurer) Get() (Config, error) {
	return c.db.GetConfig(c.instance)
}

func (c configurer) Update(update UpdateFunc) error {
	return c.db.UpdateConfig(c.instance, update)
}
