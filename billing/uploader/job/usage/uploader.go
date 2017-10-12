package usage

import (
	"context"
	"time"

	"github.com/weaveworks/service/billing/db"
	"github.com/weaveworks/service/users"
)

const (
	uploaderIDZuora = "zuora"
)

// Uploader describes a service that aggregates and uploads data to a usage consumer.
type Uploader interface {
	// ID is an unique name to represent this uploader.
	ID() string
	// Add records aggregates to be uploaded later.
	Add(ctx context.Context, orgExternalID string, from, through time.Time, aggs []db.Aggregate) error
	// Upload sends recorded aggregates.
	Upload(ctx context.Context) error
	// Handles returns whether this uploader supports the given organization.
	Handles(org users.Organization) bool
}
