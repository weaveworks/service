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
	weaveCloudURL             = "https://frontend.dev.weave.works"
	promURI                   = "https://user:7xs181ap6kabbaz3ttozt37i3ebb5e4b@frontend.dev.weave.works/api/prom"
	fluxURI                   = "https://user:7xs181ap6kabbaz3ttozt37i3ebb5e4b@frontend.dev.weave.works/api/flux"
	// weaveCloudURL             = "https://cloud.weave.works"
	// promURI                   = "http://querier.cortex.svc.cluster.local/api/prom"
	// fluxURI                   = "http://flux-api.flux.svc.cluster.local"
)

// Queries for getting resource consumption data from Prometheus
// TODO: Fix the memory query - it gives too big numbers over long time spans.
const (
	promTopCPUWorkloadsQuery    = "sort_desc(sum by (namespace, _weave_pod_name) (rate(container_cpu_usage_seconds_total{image!=''}[1h])) / ignoring(namespace, _weave_pod_name) group_left count(node_cpu{mode='idle'}))"
	promTopMemoryWorkloadsQuery = "sort_desc(sum by (namespace, _weave_pod_name) (avg_over_time(container_memory_working_set_bytes{image!=''}[1h])) / ignoring(namespace, _weave_pod_name) group_left sum (node_memory_MemTotal))"
)

// Date formats for the weekly email.
const (
	dayOfWeekFormat = "Mon"
	dateShortFormat = "Jan 2"
	dateLongFormat  = "Jan 2, 2018"
)

// WorkloadDeploymentsBar consists of a formatted Date and the total
// number of workload releases on that day.
type WorkloadDeploymentsBar struct {
	LinkTo      string
	DayOfWeek   string
	TotalCount  string
	BarHeightPx int
}

// WorkloadResourceConsumption consists of the workload name and the
// formatted percentage average cluster consumption of that workload.
type WorkloadResourceConsumption struct {
	LinkTo         string
	WorkloadName   string
	ClusterPercent string
	BarWidthPerc   float64
}

// WorkloadResourceStats blu.
type WorkloadResourceStats struct {
	Label        string
	TopConsumers []WorkloadResourceConsumption
}

// Report contains the whole of instance data summary to be sent in
// Weekly Summary emails.
type Report struct {
	FirstDay           string
	LastDay            string
	InstanceCreatedDay string
	Deployments        []WorkloadDeploymentsBar
	Resources          []WorkloadResourceStats
}

func getDeployHistoryLink(org *users.Organization, endAt time.Time, timeRange string) string {
	isoTimestamp := endAt.UTC().Format(time.RFC3339)
	return fmt.Sprintf("%s/%s/deploy/history?range=%s&timestamp=%s", weaveCloudURL, org.ExternalID, timeRange, isoTimestamp)
}

func getWorkloadSummaryLink(org *users.Organization, workloadName string) string {
	return fmt.Sprintf("%s/%s/workloads/%s/summary", weaveCloudURL, org.ExternalID, workloadName)
}

func getWorkloadReleasesHistogram(ctx context.Context, org *users.Organization, fluxURL string, startAt time.Time, endAt time.Time) ([]WorkloadDeploymentsBar, error) {
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
	weeklyMaxCount := 5
	for _, release := range data {
		t, err := time.Parse(time.RFC3339, release.Stamp)
		if err != nil {
			return nil, err
		}
		// Get a day of the week 0..6
		day := int(t.Sub(startAt).Hours() / 24)
		weeklyHistogram[day]++
		if weeklyHistogram[day] > weeklyMaxCount {
			weeklyMaxCount = weeklyHistogram[day]
		}
	}
	// org.CreatedAt = startAt.AddDate(0, 0, 3)

	// Finally convert the histogram into the appropriate array with formatted dates.
	releasesHistogram := []WorkloadDeploymentsBar{}
	for dayIndex, totalCount := range weeklyHistogram {
		dayBegin := startAt.AddDate(0, 0, dayIndex)
		dayEnd := dayBegin.AddDate(0, 0, 1)

		barHeightPx := 2 + (150.0 * totalCount / weeklyMaxCount)
		totalCount := fmt.Sprintf("%d", totalCount)

		if dayEnd.Before(org.CreatedAt) {
			barHeightPx = 0
			totalCount = "-"
		}

		releasesHistogram = append(releasesHistogram, WorkloadDeploymentsBar{
			LinkTo:      getDeployHistoryLink(org, dayEnd, "24h"),
			DayOfWeek:   dayBegin.Format(dayOfWeekFormat),
			TotalCount:  totalCount,
			BarHeightPx: barHeightPx,
		})
	}
	return releasesHistogram, nil
}

func getMostResourceIntensiveWorkloads(ctx context.Context, org *users.Organization, api v1.API, query string, EndAt time.Time) ([]WorkloadResourceConsumption, error) {
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

	maxConsumptionValue := 0.0
	for _, workload := range workloadsVector {
		if float64(workload.Value) > maxConsumptionValue {
			maxConsumptionValue = float64(workload.Value)
		}
	}

	// ... and format their name and resource consumption as rounded percentage.
	topWorkloads := []WorkloadResourceConsumption{}
	for _, workload := range workloadsVector {
		// TODO: The 'deployment' part of the name might not be valid at all times but it covers most of the cases.
		// There should be a way to get the workload in this format from namespace and pod name only, but not sure how do it now.
		workloadName := fmt.Sprintf("%s:deployment/%s", workload.Metric["namespace"], workload.Metric["_weave_pod_name"])

		topWorkloads = append(topWorkloads, WorkloadResourceConsumption{
			WorkloadName:   workloadName,
			LinkTo:         getWorkloadSummaryLink(org, workloadName),
			ClusterPercent: fmt.Sprintf("%2.2f%%", 100*float64(workload.Value)),
			BarWidthPerc:   1 + (75 * float64(workload.Value) / maxConsumptionValue),
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
func GenerateReport(org *users.Organization, endAt time.Time) (*Report, error) {
	endAt = endAt.UTC().Truncate(24 * time.Hour)               // We round down the timestamp to a day to stop at the end of previous day.
	lastDay := endAt.AddDate(0, 0, -1).Format(dateShortFormat) // Format the last day nicely (go back a day for inclusive interval).

	startAt := endAt.AddDate(0, 0, -7)          // The report will consist of full 7 days of data.
	firstDay := startAt.Format(dateShortFormat) // Format the first day nicely.

	ctx := user.InjectOrgID(context.Background(), org.ID)

	promAPI, err := getPromAPI(ctx, promURI)
	if err != nil {
		return nil, err
	}

	workloadReleasesHistogram, err := getWorkloadReleasesHistogram(ctx, org, fluxURI, startAt, endAt)
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

	instanceCreatedDay := org.CreatedAt.UTC().Format(dateLongFormat)
	if org.CreatedAt.After(startAt) {
		instanceCreatedDay = "" // Don't show instance creation info if it doesn't fall into the report time interval.
	}

	return &Report{
		FirstDay:           firstDay,
		LastDay:            lastDay,
		InstanceCreatedDay: instanceCreatedDay,
		Deployments:        workloadReleasesHistogram,
		Resources: []WorkloadResourceStats{
			WorkloadResourceStats{
				Label:        "CPU intensive workloads",
				TopConsumers: cpuIntensiveWorkloads,
			},
			WorkloadResourceStats{
				Label:        "Memory intensive workloads",
				TopConsumers: memoryIntensiveWorkloads,
			},
		},
	}, nil
}
