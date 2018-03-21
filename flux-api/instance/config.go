package instance

import (
	"time"

	"github.com/weaveworks/service/flux-api/service"
)

// Connection contains information about an instance connection.
type Connection struct {
	Last      time.Time `json:"last"`
	Connected bool      `json:"connected"`
}

// Config contains information about an instance configuration.
// TODO: remove this since it is unneeded
type Config struct {
	Connection Connection `json:"connection"`
}

// UpdateFunc takes a Connection and returns another Connection.
type UpdateFunc func(conn Connection) (Connection, error)

// DB is the instance DB interface.
type DB interface {
	UpdateConnection(instance service.InstanceID, update UpdateFunc) error
	GetConnection(instance service.InstanceID) (Connection, error)
}

type configurer struct {
	instance service.InstanceID
	db       DB
}

func (c configurer) Get() (Connection, error) {
	return c.db.GetConnection(c.instance)
}

func (c configurer) Update(update UpdateFunc) error {
	return c.db.UpdateConnection(c.instance, update)
}
