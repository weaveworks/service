package weeklySummary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	prom "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/weaveworks/common/user"
)

// prometheusClient is a specialization of the default prom.Client that extracts
// the orgID header from the given context and ensures it's forwarded to the
// querier.
type prometheusClient struct {
	client prom.Client
}

var _ prom.Client = &prometheusClient{}

func newPrometheusClient(baseURL string) (*prometheusClient, error) {
	client, err := prom.NewClient(prom.Config{
		Address: baseURL,
	})
	if err != nil {
		return nil, errors.Wrap(err, "prometheus client")
	}

	return &prometheusClient{
		client: client,
	}, nil
}

func (c *prometheusClient) URL(ep string, args map[string]string) *url.URL {
	return c.client.URL(ep, args)
}

func (c *prometheusClient) Do(ctx context.Context, r *http.Request) (*http.Response, []byte, error) {
	err := user.InjectOrgIDIntoHTTPRequest(ctx, r)
	if err != nil {
		return nil, nil, errors.Wrap(err, "inject OrgID")
	}

	return c.client.Do(ctx, r)
}

func getFluxWorkloadReleasesCount(endpoint string, startAt time.Time, endAt time.Time) int {
	after := startAt.UTC().Format(time.RFC3339)
	before := endAt.UTC().Format(time.RFC3339)
	query := fmt.Sprintf("%s/v6/history?service=<all>&simple=true&after=%s&before=%s", endpoint, after, before) // Weave Cloud Dev
	response, _ := http.Get(query)

	var data []interface{}
	json.NewDecoder(response.Body).Decode(&data)
	return len(data)
}

const (
	promTopCPUWorkloadsQuery    = "sort_desc(sum by (namespace, _weave_pod_name) (rate(container_cpu_usage_seconds_total{image!=''}[1w])) / ignoring(namespace, _weave_pod_name) group_left count(node_cpu{mode='idle'}))"
	promTopMemoryWorkloadsQuery = "sort_desc(sum by (namespace, _weave_pod_name) (avg_over_time(container_memory_working_set_bytes{image!=''}[1w])) / ignoring(namespace, _weave_pod_name) group_left sum (node_memory_MemTotal))"
)

// WorkloadResourceConsumption blu
type WorkloadResourceConsumption struct {
	Name  string
	Value string
}

func getWorkloadResourceConsumptionFromSample(sample *model.Sample) WorkloadResourceConsumption {
	return WorkloadResourceConsumption{
		Name:  fmt.Sprintf("%s:deployment/%s", sample.Metric["namespace"], sample.Metric["_weave_pod_name"]),
		Value: fmt.Sprintf("%2.2f%%", 100*float64(sample.Value)),
	}
}

func getPromAPI(ctx context.Context, endpoint string) v1.API {
	client, _ := newPrometheusClient(endpoint)
	return v1.NewAPI(client)
}

func getTopResourceIntensiveWorkloads(ctx context.Context, api v1.API, query string) []WorkloadResourceConsumption {
	workloadsVector, _ := api.Query(ctx, query, time.Now())
	topWorkloads := []WorkloadResourceConsumption{}

	for _, workload := range workloadsVector.(model.Vector)[:3] {
		topWorkloads = append(topWorkloads, getWorkloadResourceConsumptionFromSample(workload))
	}

	return topWorkloads
}

// Report blu
type Report struct {
	StartAt                  time.Time
	EndAt                    time.Time
	CPUIntensiveWorkloads    []WorkloadResourceConsumption
	MemoryIntensiveWorkloads []WorkloadResourceConsumption
	WorkloadReleasesCount    int
}

// GenerateReport blu
func GenerateReport(EndAt time.Time) Report {
	StartAt := EndAt.AddDate(0, 0, -7) // A week before
	ctx := user.InjectOrgID(context.Background(), "[ignored anyway]")
	promAPI := getPromAPI(ctx, "https://user:7xs181ap6kabbaz3ttozt37i3ebb5e4b@frontend.dev.weave.works/api/prom") // Weave Cloud Dev
	fluxAPI := "https://user:7xs181ap6kabbaz3ttozt37i3ebb5e4b@frontend.dev.weave.works/api/flux"                  // Weave Cloud Dev

	return Report{
		CPUIntensiveWorkloads:    getTopResourceIntensiveWorkloads(ctx, promAPI, promTopCPUWorkloadsQuery),
		MemoryIntensiveWorkloads: getTopResourceIntensiveWorkloads(ctx, promAPI, promTopMemoryWorkloadsQuery),
		WorkloadReleasesCount:    getFluxWorkloadReleasesCount(fluxAPI, StartAt, EndAt),
		StartAt:                  StartAt,
		EndAt:                    EndAt,
	}
}
