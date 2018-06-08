package db

import (
	"time"

	"github.com/pkg/errors"
	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/notification-eventmanager/db/postgres"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/notification-eventmanager/utils"
)

// DB is the interface for the database.
type DB interface {
	CheckInstanceDefaults(instanceID string, defaultReceiver types.Receiver) error

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

	CreateEvent(event types.Event, featureFlags []string) (eventID string, err error)
	GetEvents(instanceID string, fields, eventTypes []string, before, after time.Time, limit, offset int) ([]*types.Event, error)
}

// New creates a new database.
func New(cfg dbconfig.Config) (DB, error) {
	scheme, dataSourceName, migrationsDir, err := cfg.Parameters()
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse URI")
	}

	switch scheme {
	case "postgres":
		return postgres.New(dataSourceName, migrationsDir)
	default:
		return nil, errors.Errorf("Unknown database type: %s", scheme)
	}
}
