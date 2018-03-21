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

// ConnectionDB is a Connection database.
type ConnectionDB interface {
	Get(service.InstanceID) (Connection, error)
	Connect(service.InstanceID, time.Time) error
	Disconnect(service.InstanceID, time.Time) error
}

type configurer struct {
	instance service.InstanceID
	db       ConnectionDB
}

func (c configurer) Get() (Connection, error) {
	return c.db.Get(c.instance)
}

func (c configurer) Connect(t time.Time) error {
	return c.db.Connect(c.instance, t)
}
