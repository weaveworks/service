package usage

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/api/servicecontrol/v1"

	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/common/constants/billing"
	"github.com/weaveworks/service/common/gcp/control"
	"github.com/weaveworks/service/users"
)

// GCP implements usage upload to the Google Cloud Platform through the Google Service Control API.
type GCP struct {
	client *control.Client
	ops    []*servicecontrol.Operation
}

// NewGCP creates a client for the Service Control API.
func NewGCP(cfg control.Config) (*GCP, error) {
	cl, err := control.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &GCP{client: cl}, nil
}

// ID returns an unique uploader id.
func (g *GCP) ID() string {
	return "gcp"
}

// Add collects node-seconds aggregates.
func (g *GCP) Add(ctx context.Context, org users.Organization, from, through time.Time, aggs []db.Aggregate) error {
	for _, agg := range aggs {
		if agg.AmountType != billing.UsageNodeSeconds {
			continue
		}
		g.ops = append(g.ops, &servicecontrol.Operation{
			OperationId:   g.client.OperationID(strconv.Itoa(agg.ID)), // same id for same operation helps deduplication
			OperationName: "HourlyUsageUpload",                        // can be selected freely
			ConsumerId:    org.GCPConsumerID,
			StartTime:     from.Format(time.RFC3339Nano),
			EndTime:       through.Format(time.RFC3339Nano),
			MetricValueSets: []*servicecontrol.MetricValueSet{{
				MetricName: fmt.Sprintf("google.weave.works/%s_nodes", org.GCPSubscriptionLevel),
				MetricValues: []*servicecontrol.MetricValue{{
					Int64Value: &agg.AmountValue,
				}},
			}},
		})
	}
	return nil
}

// Upload sends the usage to the Service Control API as metrics.
func (g *GCP) Upload(ctx context.Context) error {
	return g.client.Report(ctx, g.ops)
}

// IsSupported doesn't yet know how to determine supported organizations.
func (g *GCP) IsSupported(org users.Organization) bool {
	return false
}
