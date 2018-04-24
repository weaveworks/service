package db

import (
	"flag"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/weaveworks/service/notification-eventmanager/db/postgres"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/notification-eventmanager/utils"
)

// Config configures the database.
type Config struct {
	URI           string
	MigrationsDir string
}

// RegisterFlags adds the flags required to configure this to the given FlagSet.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	flag.StringVar(&cfg.URI, "database.uri", "", "URI where the database can be found")
	flag.StringVar(&cfg.MigrationsDir, "database.migrations", "", "Path where the database migration files can be found")
}

// DB is the interface for the database.
type DB interface {
	CreateEventType(tx *utils.Tx, e types.EventType) error
	UpdateEventType(tx *utils.Tx, e types.EventType) error
	DeleteEventType(tx *utils.Tx, eventTypeName string) error
	SyncEventTypes(eventTypes map[string]types.EventType) error
	ListEventTypes(tx *utils.Tx, featureFlags []string) ([]types.EventType, error)

	CreateReceiver(receiver types.Receiver, instanceID string) (string, error)
	UpdateReceiver(receiver types.Receiver, instanceID string, featureFlags []string) error
	DeleteReceiver(instanceID, receiverID string) (int64, error)
	GetReceiver(instanceID, receiverID string, featureFlags []string, omitHiddenEventTypes bool) (types.Receiver, error)
	GetReceiversForEvent(event types.Event) ([]types.Receiver, error)
	ListReceivers(instanceID string) ([]types.Receiver, error)

	CreateEvent(event types.Event, featureFlags []string) error
	GetEvents(instanceID string, fields, eventTypes []string, before, after time.Time, limit, offset int) ([]*types.Event, error)
}

// New creates a new database.
func New(cfg Config) (DB, error) {
	u, err := url.Parse(cfg.URI)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse URI")
	}

	switch u.Scheme {
	case "postgres":
		return postgres.New(cfg.URI, cfg.MigrationsDir)
	default:
		return nil, errors.Errorf("Unknown database type: %s", u.Scheme)
	}
}
