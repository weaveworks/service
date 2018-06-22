package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/servicecontrol/v1"

	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/common/constants/billing"
	"github.com/weaveworks/service/common/gcp/control"
	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/users"
)

// GCP implements usage upload to the Google Cloud Platform through the Google Service Control API.
type GCP struct {
	client control.API
	ops    []*servicecontrol.Operation
}

// NewGCP instantiates a GCP usage uploader.
func NewGCP(client control.API) *GCP {
	return &GCP{client: client}
}

// ID returns an unique uploader id.
func (g *GCP) ID() string {
	return "gcp"
}

// Reset removes all current operations.
func (g *GCP) Reset() {
	g.ops = nil
}

// Add collects node-seconds aggregates.
func (g *GCP) Add(ctx context.Context, org users.Organization, from, through time.Time, aggs []db.Aggregate) error {
	for _, agg := range aggs {
		if agg.AmountType != billing.UsageNodeSeconds {
			continue
		}
		value := agg.AmountValue
		g.ops = append(g.ops, &servicecontrol.Operation{
			OperationId:   g.client.OperationID(strconv.Itoa(agg.ID)), // same id for same operation helps deduplication
			OperationName: "HourlyUsageUpload",                        // can be selected freely
			ConsumerId:    org.GCP.ConsumerID,
			StartTime:     agg.BucketStart.Format(time.RFC3339Nano),
			EndTime:       agg.BucketStart.Add(1 * time.Hour).Format(time.RFC3339Nano), // bucket size is always 1h
			MetricValueSets: []*servicecontrol.MetricValueSet{{
				MetricName: fmt.Sprintf("google.weave.works/%s_nodes", org.GCP.SubscriptionLevel),
				MetricValues: []*servicecontrol.MetricValue{{
					Int64Value: &value,
				}},
			}},
		})
	}
	return nil
}

// Upload sends the usage to the Service Control API as metrics.
func (g *GCP) Upload(ctx context.Context, id string) error {
	bs, _ := json.Marshal(g.ops)
	log.Infof("Uploading GCP usage: %s", bs)
	return g.client.Report(ctx, g.ops)
}

// IsSupported only picks organizations that have an activated GCP account
func (g *GCP) IsSupported(org users.Organization) bool {
	// Note that users.GetBillableOrganizations should already check for all of these except
	// GCP != nil. Better safe than sorry.
	return org.GCP != nil && org.GCP.Activated && org.GCP.SubscriptionStatus == string(partner.Active)
}

// ThroughTime truncates to the hour. We always want to upload everything that has been fully aggregated.
func (g *GCP) ThroughTime(now time.Time) time.Time {
	return now.Truncate(1 * time.Hour)
}
