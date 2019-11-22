package weeklyreports

import (
	"fmt"
	"net/url"
	"time"
)

// Date formats for the weekly email.
const (
	dayOfWeekFormat = "Mon"
	dateDayFormat   = "2"
	dateShortFormat = "Jan 2"
	dateLongFormat  = "January 2nd, 2006"
)

// EmailSummary contains all the data for rendering the weekly summary report in the email.
type EmailSummary struct {
	HasData      bool
	DateInterval string
	Organization OrganizationInfo
	Deployments  WorkloadDeploymentsHistogram
	Resources    WorkloadResourcesChart
}

// OrganizationInfo contains the basic organizations information.
type OrganizationInfo struct {
	Name        string
	CreationDay string
	HomepageURL string
	StatusURL   string
	DeleteURL   string
}

// WorkloadDeploymentsHistogram for rendering a deployments histogram.
type WorkloadDeploymentsHistogram struct {
	HasData   bool
	LinkTo    string
	Histogram []WorkloadDeploymentsBar
}

// WorkloadDeploymentsBar contains info for rendering a single deployments count vertical bar.
type WorkloadDeploymentsBar struct {
	LinkTo      string
	DayOfWeek   string
	TotalCount  string
	BarHeightPx int
}

// WorkloadResourcesChart describes the resource consumption data to be rendered.
type WorkloadResourcesChart struct {
	HasData      bool
	LinkTo       string
	TopConsumers []WorkloadResourceTopConsumers
}

// WorkloadResourceTopConsumers describes a list of top consuming workloads for a fixed resource.
type WorkloadResourceTopConsumers struct {
	Label     string
	LinkTo    string
	Workloads []WorkloadResourceConsumptionInfo
}

// WorkloadResourceConsumptionInfo contains info for rendering a single horizontal consumption bar of a single resource.
type WorkloadResourceConsumptionInfo struct {
	LinkTo            string
	WorkloadNameShort string
	WorkloadNameFull  string
	ClusterPercent    string
	BarWidthPercent   float64
}

func getDeployHistoryLink(organizationURL string, endAt time.Time, timeRange string) string {
	isoTimestamp := endAt.UTC().Format(time.RFC3339)
	return fmt.Sprintf("%s/deploy/history?range=%s&timestamp=%s", organizationURL, timeRange, isoTimestamp)
}

func getWorkloadSummaryLink(organizationURL string, workloadName string) string {
	return fmt.Sprintf("%s/workloads/%s/summary", organizationURL, url.QueryEscape(workloadName))
}

func getDeployLink(organizationURL string) string {
	return fmt.Sprintf("%s/deploy", organizationURL)
}

func getWorkloadResourcesLink(organizationURL string) string {
	return fmt.Sprintf("%s/monitor/dashboard/workload-resources?namespace=all", organizationURL)
}

func getDeleteOrganizationLink(organizationURL string) string {
	return fmt.Sprintf("%s/org/delete", organizationURL)
}

func truncateString(s string, cap int) string {
	if len(s) > cap {
		s = s[:cap-3] + "..."
	}
	return s
}

func generateDeploymentsHistogram(report *Report, organizationURL string) WorkloadDeploymentsHistogram {
	// To normalize the deployments bars to a fixed height, we need to divide them by the max number.
	maxDeploymentsCount := 0
	for _, deploymentsCount := range report.DeploymentsPerDay {
		if deploymentsCount > maxDeploymentsCount {
			maxDeploymentsCount = deploymentsCount
		}
	}
	// Here we put it at least to 1 to avoid division by zero in case of no deployments for a whole week.
	deploymentsCap := maxDeploymentsCount
	if deploymentsCap == 0 {
		deploymentsCap = 1
	}

	releasesHistogram := []WorkloadDeploymentsBar{}
	for dayIndex, totalCount := range report.DeploymentsPerDay {
		dayBegin := report.StartAt.AddDate(0, 0, dayIndex)
		dayEnd := dayBegin.AddDate(0, 0, 1)

		// Render a very thin bar for 0 deployments; max bar height will be 150px, the rest linearly proportional.
		barHeightPx := 2 + (150.0 * totalCount / deploymentsCap)
		linkTo := getDeployHistoryLink(organizationURL, dayEnd, "24h")
		totalCount := fmt.Sprintf("%d", totalCount)

		// Render an empty bar if the organization wasn't created at that day yet.
		if dayEnd.Before(report.Organization.CreatedAt) {
			linkTo = ""
			totalCount = "-"
			barHeightPx = 0
		}

		releasesHistogram = append(releasesHistogram, WorkloadDeploymentsBar{
			DayOfWeek:   dayBegin.Format(dayOfWeekFormat),
			LinkTo:      linkTo,
			TotalCount:  totalCount,
			BarHeightPx: barHeightPx,
		})
	}

	return WorkloadDeploymentsHistogram{
		LinkTo:    getDeployLink(organizationURL),
		HasData:   maxDeploymentsCount > 0,
		Histogram: releasesHistogram,
	}
}

func generateResourcesChart(report *Report, organizationURL string) WorkloadResourcesChart {
	cpuTopConsumers := generateResourceBars(report.CPUIntensiveWorkloads, organizationURL)
	memoryTopConsumers := generateResourceBars(report.MemoryIntensiveWorkloads, organizationURL)
	workloadResourcesLink := getWorkloadResourcesLink(organizationURL)

	return WorkloadResourcesChart{
		LinkTo:  workloadResourcesLink,
		HasData: len(cpuTopConsumers) > 0 || len(memoryTopConsumers) > 0,
		TopConsumers: []WorkloadResourceTopConsumers{
			{Label: "CPU", LinkTo: workloadResourcesLink, Workloads: cpuTopConsumers},
			{Label: "Memory", LinkTo: workloadResourcesLink, Workloads: memoryTopConsumers},
		},
	}
}

func generateResourceBars(workloads []WorkloadResourceConsumptionRaw, organizationURL string) []WorkloadResourceConsumptionInfo {
	// To normalize the resource consumption bars to a fixed width, we need to divide them by the longest bar.
	// The initial value is set just above 0 to avoid division by zero in case it happens to be nil for all workloads.
	maxClusterConsumption := 0.00001
	for _, workload := range workloads {
		clusterConsumption := float64(workload.ClusterConsumption)
		if clusterConsumption > maxClusterConsumption {
			maxClusterConsumption = clusterConsumption
		}
	}

	topWorkloads := []WorkloadResourceConsumptionInfo{}
	for _, workload := range workloads {
		// Render a very thin bar for min resource usage; bar width will be in the range (5, 70], the rest linearly proportional.
		clusterConsumption := float64(workload.ClusterConsumption)
		addedPercent := float64(5) // In the event a cluster has a consumption near 0, we want to at least show some visual indication of the bar's existence - so we add 5 arbitrary percent.
		scaleFactor := float64(65) // We don't want this number to be 100 because there needs to be 30% or so of the width (including the addedPercent) available for the trailing percentage text that's on the same line.
		percentageOfMaximum := clusterConsumption / maxClusterConsumption
		barWidthPercent := addedPercent + (scaleFactor * percentageOfMaximum)

		clusterPercent := fmt.Sprintf("%2.2f%%", 100*clusterConsumption)

		topWorkloads = append(topWorkloads, WorkloadResourceConsumptionInfo{
			WorkloadNameFull:  workload.WorkloadName,
			WorkloadNameShort: truncateString(workload.WorkloadName, 35),
			LinkTo:            getWorkloadSummaryLink(organizationURL, workload.WorkloadName),
			ClusterPercent:    clusterPercent,
			BarWidthPercent:   barWidthPercent,
		})
	}
	return topWorkloads
}

func getReportInterval(report *Report) string {
	// Format the last day nicely (go back a day for inclusive interval).
	lastDay := report.EndAt.AddDate(0, 0, -1)
	firstDay := report.StartAt
	if lastDay.Month() == firstDay.Month() {
		// If months match don't repeat it
		return fmt.Sprintf("%s–%s", firstDay.Format(dateShortFormat), lastDay.Format(dateDayFormat))
	}
	return fmt.Sprintf("%s – %s", firstDay.Format(dateShortFormat), lastDay.Format(dateShortFormat))
}

func getOrganizationCreationDayIfRecent(report *Report) string {
	organizationCreatedAt := report.Organization.CreatedAt.UTC()
	// Return organization creation date only if it falls in this report's interval.
	if organizationCreatedAt.After(report.StartAt) {
		return organizationCreatedAt.Format(dateLongFormat)
	}
	return ""
}

// EmailSummaryFromReport returns the weekly summary report in the format directly consumable by email templates.
func EmailSummaryFromReport(report *Report, organizationURL string) *EmailSummary {
	deployments := generateDeploymentsHistogram(report, organizationURL)
	resources := generateResourcesChart(report, organizationURL)

	return &EmailSummary{
		Organization: OrganizationInfo{
			CreationDay: getOrganizationCreationDayIfRecent(report),
			Name:        report.Organization.Name,
			HomepageURL: organizationURL,
			StatusURL:   organizationURL,
			DeleteURL:   getDeleteOrganizationLink(organizationURL),
		},
		DateInterval: getReportInterval(report),
		HasData:      deployments.HasData || resources.HasData,
		Deployments:  deployments,
		Resources:    resources,
	}
}
