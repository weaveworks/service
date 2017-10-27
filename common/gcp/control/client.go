package control

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/satori/go.uuid"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/servicecontrol/v1"

	"github.com/weaveworks/common/instrument"
)

var clientRequestCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: "google",
	Subsystem: "servicecontrol_client",
	Name:      "request_duration_seconds",
	Help:      "Response time of Google Service Control API requests.",
	Buckets:   prometheus.DefBuckets,
})

func init() {
	clientRequestCollector.Register()
}

// Client provides access to the Google Service Control API
//
// https://cloud.google.com/service-control/overview
// https://cloud.google.com/service-control/reporting-billing-metrics
type Client struct {
	svc         *servicecontrol.ServicesService
	serviceName string
}

// NewClient returns a Client accessing the Service Control API. It uses
// oauth2 for authentication.
func NewClient(cfg Config) (*Client, error) {
	jsonKey, err := ioutil.ReadFile(cfg.ServiceAccountKeyFile)
	if err != nil {
		return nil, err
	}

	// Create oauth2 HTTP client from the given service account key JSON
	jwtConf, err := google.JWTConfigFromJSON(jsonKey, servicecontrol.ServicecontrolScope)
	if err != nil {
		return nil, err
	}
	cl := jwtConf.Client(context.Background())
	cl.Timeout = cfg.Timeout

	s, err := servicecontrol.New(cl)
	if err != nil {
		return nil, err
	}
	s.BasePath = cfg.URL
	if s.BasePath[len(s.BasePath)-1] != '/' {
		s.BasePath += "/"
	}
	return &Client{
		svc:         s.Services,
		serviceName: cfg.ServiceName,
	}, nil
}

// Report sends off a slice of operations.
// It requires the `servicemanagement.services.report` permission.
func (c *Client) Report(ctx context.Context, operations []*servicecontrol.Operation) error {
	req := &servicecontrol.ReportRequest{
		Operations: operations,
	}

	return instrument.CollectedRequest(ctx, "Report", clientRequestCollector, nil, func(_ context.Context) error {
		res, err := c.svc.Report(c.serviceName, req).Do()
		if err != nil {
			return err
		}

		// Catch partial errors
		if len(res.ReportErrors) != 0 {
			return errors.Wrapf(ReportErrors(res.ReportErrors), "control: errors during metric processing")
		}

		return nil
	})
}

// ReportErrors implements error.
type ReportErrors []*servicecontrol.ReportError

// Error concatenates report errors up to a handful.
func (r ReportErrors) Error() string {
	tot := len(r)
	errs := []string{}
	for i, e := range r {
		if i >= 5 {
			errs = append(errs, fmt.Sprintf("%d errors in total", tot))
			break
		}
		errs = append(errs, fmt.Sprintf("[%v]: (%v) %v", e.OperationId, e.Status.Code, e.Status.Message))
	}
	return strings.Join(errs, " / ")
}

// OperationID creates a UUIDv5 operation id.
//
// From the documentation:
//
//   UUID version 4 is recommended, though not required.
//   In scenarios where an operation is computed from existing
//   information and an idempotent id is desirable for deduplication
//   purpose, UUID version 5 is recommended.
//
func (c *Client) OperationID(name string) string {
	return uuid.NewV5(uuid.Nil, name).String()
}
