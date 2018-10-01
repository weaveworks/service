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
)

const (
	resourceWorkloadsMaxShown = 3
	promURI                   = "http://querier.cortex.svc.cluster.local/api/prom"
	fluxURI                   = "http://flux-api.flux.svc.cluster.local"
)

// Queries for getting resource consumption data from Prometheus
// TODO: Fix the memory query - it gives too big numbers over long time spans.
const (
	promTopCPUWorkloadsQuery    = "sort_desc(sum by (namespace, _weave_pod_name) (rate(container_cpu_usage_seconds_total{image!=''}[1w])) / ignoring(namespace, _weave_pod_name) group_left count(node_cpu{mode='idle'}))"
	promTopMemoryWorkloadsQuery = "sort_desc(sum by (namespace, _weave_pod_name) (avg_over_time(container_memory_working_set_bytes{image!=''}[1w])) / ignoring(namespace, _weave_pod_name) group_left sum (node_memory_MemTotal))"
)

// Date formats for the weekly email.
const (
	dateWeekFormat  = "Jan 2 (Mon)"
	dateShortFormat = "Jan 2"
)

// WorkloadReleasesCount consists of a formatted Date and the total
// number of workload releases on that day.
type WorkloadReleasesCount struct {
	Day   string
	Total int
}

// WorkloadResourceConsumption consists of the workload name and the
// formatted percentage average cluster consumption of that workload.
type WorkloadResourceConsumption struct {
	Name  string
	Value string
}

// Report contains the whole of instance data summary to be sent in
// Weekly Summary emails.
type Report struct {
	FirstDay                 string
	LastDay                  string
	CPUIntensiveWorkloads    []WorkloadResourceConsumption
	MemoryIntensiveWorkloads []WorkloadResourceConsumption
	WorkloadReleasesCounts   []WorkloadReleasesCount
}

func getWorkloadReleasesCounts(ctx context.Context, fluxURL string, startAt time.Time, endAt time.Time) ([]WorkloadReleasesCount, error) {
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
	weeklyHistogram := [7]int{}
	for _, release := range data {
		t, err := time.Parse(time.RFC3339, release.Stamp)
		if err != nil {
			return nil, err
		}
		// Get a day of the week 0..6
		day := int(t.Sub(startAt).Hours() / 24)
		weeklyHistogram[day]++
	}

	// Finally convert the histogram into the appropriate array with formatted dates.
	releasesCounts := []WorkloadReleasesCount{}
	for day, total := range weeklyHistogram {
		releasesCounts = append(releasesCounts, WorkloadReleasesCount{
			Day:   startAt.AddDate(0, 0, day).Format(dateWeekFormat),
			Total: total,
		})
	}
	return releasesCounts, nil
}

func getMostResourceIntensiveWorkloads(ctx context.Context, api v1.API, query string, EndAt time.Time) ([]WorkloadResourceConsumption, error) {
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

	// ... and format their name and resource consumption as rounded percentage.
	topWorkloads := []WorkloadResourceConsumption{}
	for _, workload := range workloadsVector {
		topWorkloads = append(topWorkloads, WorkloadResourceConsumption{
			// TODO: The 'deployment' part of the name might not be valid at all times but it covers most of the cases.
			// There should be a way to get the workload in this format from namespace and pod name only, but not sure how do it now.
			Name:  fmt.Sprintf("%s:deployment/%s", workload.Metric["namespace"], workload.Metric["_weave_pod_name"]),
			Value: fmt.Sprintf("%2.2f%%", 100*float64(workload.Value)),
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

// GenerateReport returns the weekly summary report in the format directly consumable by email templates.
func GenerateReport(orgID string, endAt time.Time) (*Report, error) {
	endAt = endAt.UTC().Truncate(24 * time.Hour)               // We round down the timestamp to a day to stop at the end of previous day.
	lastDay := endAt.AddDate(0, 0, -1).Format(dateShortFormat) // Format the last day nicely (go back a day for inclusive interval).

	startAt := endAt.AddDate(0, 0, -7)          // The report will consist of full 7 days of data.
	firstDay := startAt.Format(dateShortFormat) // Format the first day nicely.

	ctx := user.InjectOrgID(context.Background(), orgID)

	promAPI, err := getPromAPI(ctx, promURI)
	if err != nil {
		return nil, err
	}

	workloadReleasesCounts, err := getWorkloadReleasesCounts(ctx, fluxURI, startAt, endAt)
	if err != nil {
		return nil, err
	}
	cpuIntensiveWorkloads, err := getMostResourceIntensiveWorkloads(ctx, promAPI, promTopCPUWorkloadsQuery, endAt)
	if err != nil {
		return nil, err
	}
	memoryIntensiveWorkloads, err := getMostResourceIntensiveWorkloads(ctx, promAPI, promTopMemoryWorkloadsQuery, endAt)
	if err != nil {
		return nil, err
	}

	return &Report{
		CPUIntensiveWorkloads:    cpuIntensiveWorkloads,
		MemoryIntensiveWorkloads: memoryIntensiveWorkloads,
		WorkloadReleasesCounts:   workloadReleasesCounts,
		FirstDay:                 firstDay,
		LastDay:                  lastDay,
	}, nil
}
