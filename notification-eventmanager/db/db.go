package db

import (
	"github.com/weaveworks/service/notification-eventmanager/db/postgres"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

type DB interface {
	CreateEvent(types.Event) (id string, err error)
	SaveAttachments(event_id string, attachments []types.Attachment) error
	ListEvents(instanceID string, offset, lenth int) ([]types.Event, error)
}

func New(uri string, migrationsDir string) (DB, error) {
	db, err := postgres.New(uri, migrationsDir)

	if err != nil {
		return nil, err
	}

	return db, nil
}
