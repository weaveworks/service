package usage

import (
	"context"
	"time"

	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/users"
)

// Uploader describes a service that aggregates and uploads data to a usage consumer.
type Uploader interface {
	// ID is an unique name to represent this uploader.
	// IMPORTANT: when implementing the Uploader interface, values returned by ID() need to be added as
	// valid values in the DB's enum uploader_type. See also: 005 and 006 in billing-api/db/migrations/
	ID() string
	// Add records aggregates to be uploaded later.
	Add(ctx context.Context, org users.Organization, from, through time.Time, aggs []db.Aggregate) error
	// Upload sends recorded aggregates.
	Upload(ctx context.Context) error
	// Reset creates a fresh report.
	Reset()
	// IsSupported returns whether this uploader handles the given organization.
	IsSupported(org users.Organization) bool
	// ThroughTime returns the upper bound we want to upload usage until.
	ThroughTime(now time.Time) time.Time
}
