package weeklysummary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/users"
)

const (
	resourceWorkloadsMaxShown = 3
	promURI                   = "https://user:7xs181ap6kabbaz3ttozt37i3ebb5e4b@frontend.dev.weave.works/api/prom"
	fluxURI                   = "https://user:7xs181ap6kabbaz3ttozt37i3ebb5e4b@frontend.dev.weave.works/api/flux"
	// promURI                   = "http://querier.cortex.svc.cluster.local/api/prom"
	// fluxURI                   = "http://flux-api.flux.svc.cluster.local"
)

// Queries for getting resource consumption data from Prometheus
// TODO: Fix the memory query - it gives too big numbers over long time spans.
const (
	promTopCPUWorkloadsQuery    = "sort_desc(sum by (namespace, _weave_pod_name) (rate(container_cpu_usage_seconds_total{image!=''}[1h])) / ignoring(namespace, _weave_pod_name) group_left count(node_cpu{mode='idle'}))"
	promTopMemoryWorkloadsQuery = "sort_desc(sum by (namespace, _weave_pod_name) (avg_over_time(container_memory_working_set_bytes{image!=''}[1h])) / ignoring(namespace, _weave_pod_name) group_left sum (node_memory_MemTotal))"
)

// Unformated consumption data returned by Prometheus.
type workloadResourceConsumption struct {
	WorkloadName       string
	ClusterConsumption float64
}

// Report contains all the raw summary data for the weekly emails.
type Report struct {
	Organization             *users.Organization
	GeneratedAt              time.Time
	StartAt                  time.Time
	EndAt                    time.Time
	DeploymentsPerDay        []int
	CPUIntensiveWorkloads    []workloadResourceConsumption
	MemoryIntensiveWorkloads []workloadResourceConsumption
}

func getWorkloadDeploymentsPerDay(ctx context.Context, org *users.Organization, fluxURL string, startAt time.Time, endAt time.Time) ([]int, error) {
	// TODO: This should probably be handled via a dedicated Flux client, similar to the Prometheus one.
	after := startAt.UTC().Format(time.RFC3339)
	before := endAt.UTC().Format(time.RFC3339)
	query := fmt.Sprintf("%s/v6/history?service=<all>&simple=true&after=%s&before=%s", fluxURL, after, before)
	request, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return nil, err
	}
	request = request.WithContext(ctx)
	user.InjectOrgIDIntoHTTPRequest(ctx, request)
	client := http.DefaultClient
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	// Read only the timestamps from all the deploy events.
	var data []struct{ Stamp string }
	json.NewDecoder(response.Body).Decode(&data)

	// Build a histogram of releases counts per every day of the week.
	// TODO: We should probably filter out anything but `Sync` events here for more accurate numbers.
	weeklyHistogram := make([]int, 7)
	for _, release := range data {
		t, err := time.Parse(time.RFC3339, release.Stamp)
		if err != nil {
			return nil, err
		}
		// Get a day of the week 0..6
		day := int(t.Sub(startAt).Hours() / 24)
		weeklyHistogram[day]++
	}
	return weeklyHistogram, nil
}

func getMostResourceIntensiveWorkloads(ctx context.Context, org *users.Organization, api v1.API, query string, EndAt time.Time) ([]workloadResourceConsumption, error) {
	// Get the sorted list of workloads based on the query.
	workloadsSeries, err := api.Query(ctx, query, EndAt)
	if err != nil {
		return nil, err
	}

	// Get at most `resourceWorkloadsMaxShown` workloads from the top ...
	workloadsVector := workloadsSeries.(model.Vector)
	if len(workloadsVector) > resourceWorkloadsMaxShown {
		workloadsVector = workloadsVector[:resourceWorkloadsMaxShown]
	}

	// ... and store that data, together with the workload names.
	topWorkloads := []workloadResourceConsumption{}
	for _, workload := range workloadsVector {
		// TODO: The 'deployment' part of the name might not be valid at all times but it covers most of the cases.
		// There should be a way to get the workload in this format from namespace and pod name only, but not sure how do it now.
		workloadName := fmt.Sprintf("%s:deployment/%s", workload.Metric["namespace"], workload.Metric["_weave_pod_name"])
		topWorkloads = append(topWorkloads, workloadResourceConsumption{
			WorkloadName:       workloadName,
			ClusterConsumption: float64(workload.Value),
		})
	}
	return topWorkloads, nil
}

func getPromAPI(ctx context.Context, uri string) (v1.API, error) {
	client, err := common.NewPrometheusClient(uri)
	if err != nil {
		return nil, err
	}
	return v1.NewAPI(client), nil
}

// GenerateReport returns the raw weekly report data for the organization.
func GenerateReport(org *users.Organization, endAt time.Time) (*Report, error) {
	endAt = endAt.UTC().Truncate(24 * time.Hour) // We round down the timestamp to a day to stop at the end of previous day.
	startAt := endAt.AddDate(0, 0, -7)           // The report will consist of full 7 days of data.

	ctx := user.InjectOrgID(context.Background(), org.ID)

	promAPI, err := getPromAPI(ctx, promURI)
	if err != nil {
		return nil, err
	}

	deploymentsPerDay, err := getWorkloadDeploymentsPerDay(ctx, org, fluxURI, startAt, endAt)
	if err != nil {
		return nil, err
	}
	cpuIntensiveWorkloads, err := getMostResourceIntensiveWorkloads(ctx, org, promAPI, promTopCPUWorkloadsQuery, endAt)
	if err != nil {
		return nil, err
	}
	memoryIntensiveWorkloads, err := getMostResourceIntensiveWorkloads(ctx, org, promAPI, promTopMemoryWorkloadsQuery, endAt)
	if err != nil {
		return nil, err
	}

	return &Report{
		Organization:             org,
		GeneratedAt:              time.Now(),
		StartAt:                  startAt,
		EndAt:                    endAt,
		DeploymentsPerDay:        deploymentsPerDay,
		CPUIntensiveWorkloads:    cpuIntensiveWorkloads,
		MemoryIntensiveWorkloads: memoryIntensiveWorkloads,
	}, nil
}
