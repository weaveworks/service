package routes

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/trial"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
)

type totalSums map[string]int64
type monthSums map[time.Month]totalSums
type instanceMonthSums map[string]monthSums

// totalSums is our storage mechanism and is defined as int64 values
// node-usage is going to be a float, but needs to be stores as int64
// So, we preserve a digit for every power of ten
const nodeUsagePrecision int = 1000000

// healthCheck handles a very simple health check
func (a *API) healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// ExportOrgsAndUsageAsCSV loads organizations, usage data for them, and formats this data as a CSV file.
func (a *API) ExportOrgsAndUsageAsCSV(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	from, to, err := parseRange(fromStr, toStr)
	if err != nil {
		renderError(w, r, err)
		return
	}

	logger := user.LogWith(r.Context(), logging.Global()).WithFields(logging.Fields{"from": fromStr, "to": toStr})
	logger.Infoln("csv export: getting summary from users service")
	summary, err := a.Users.GetSummary(r.Context(), &users.Empty{})
	if err != nil {
		logger.WithField("err", err).Errorln("csv export: failed to get summary from users service")
		renderError(w, r, err)
		return
	}
	logger.Infoln("csv export: successfully got summary from users service")

	orgIDs := orgIDs(summary.Entries)
	logger.WithField("num_orgs", len(orgIDs)).Infoln("csv export: getting usage from billing-db")
	sums, err := a.DB.GetMonthSums(r.Context(), orgIDs, from, to)

	if err != nil {
		logger.WithField("err", err).Errorln("csv export: failed to get usage from billing-db")
		renderError(w, r, err)
		return
	}
	logger.Infoln("csv export: successfully got usage from billing-db")

	instanceMonthSums, amountTypesMap := processSums(sums)
	amountTypes, _ := processAmountTypes(amountTypesMap)
	months := months(from, to)

	csvLines := [][]string{header(months, amountTypes)}
	for _, entry := range summary.Entries {
		usages := usages(instanceMonthSums[entry.OrgID], months, amountTypes)
		csvLines = append(csvLines, toCSVLine(entry, usages))
	}
	renderCSV(w, csvLines, logger)
}

// Generates the header for the CSV file: first block is hard-coded names;
// second block is auto-generated monthly usage stat headers
func header(months []time.Month, amountTypes []string) []string {
	header := []string{
		"TeamExternalID", "TeamName", "OrgID", "OrgExternalID", "OrgName", "OrgCreatedAt",
		"Emails", "FirstSeenConnectedAt", "Platform", "Environment",
		"TrialExpiresAt", "TrialPendingExpiryNotifiedAt", "TrialExpiredNotifiedAt",
		"BillingEnabled", "RefuseDataAccess", "RefuseDataUpload", "ZuoraAccountNumber", "ZuoraAccountCreatedAt",
		"GCPAccountExternalID", "GCPAccountCreatedAt", "GCPAccountSubscriptionLevel", "GCPAccountSubscriptionStatus",
	}
	for _, month := range months {
		for _, amountType := range amountTypes {
			header = append(header, fmt.Sprintf("%v in %v", amountType, month))
		}
	}
	return header
}

func usages(usage monthSums, months []time.Month, amountTypes []string) []int64 {
	var values []int64
	for _, month := range months {
		for _, amountType := range amountTypes {
			values = append(values, usage[month][amountType])
		}
	}
	return values
}

func toCSVLine(entry *users.SummaryEntry, usages []int64) []string {
	line := []string{
		entry.TeamExternalID,
		entry.TeamName,
		entry.OrgID,
		entry.OrgExternalID,
		entry.OrgName,
		toString(&entry.OrgCreatedAt),
		strings.Join(entry.Emails, " ; "),
		toString(entry.FirstSeenConnectedAt),
		entry.Platform,
		entry.Environment,
		toString(&entry.TrialExpiresAt),
		toString(entry.TrialPendingExpiryNotifiedAt),
		toString(entry.TrialExpiredNotifiedAt),
		strconv.FormatBool(entry.BillingEnabled),
		strconv.FormatBool(entry.RefuseDataAccess),
		strconv.FormatBool(entry.RefuseDataUpload),
		entry.ZuoraAccountNumber,
		toString(entry.ZuoraAccountCreatedAt),
		entry.GCPAccountExternalID,
		toString(&entry.GCPAccountCreatedAt),
		entry.GCPAccountSubscriptionLevel,
		entry.GCPAccountSubscriptionStatus,
	}
	for _, usage := range usages {
		line = append(line, strconv.FormatInt(usage, 10))
	}
	return line
}

func toString(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func renderCSV(w http.ResponseWriter, csvLines [][]string, logger logging.Interface) {
	buffer := &bytes.Buffer{}
	csvWriter := csv.NewWriter(buffer)
	csvWriter.WriteAll(csvLines)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=billing.csv")
	w.WriteHeader(http.StatusOK)
	bytesCSV := buffer.Bytes()
	bytesWritten, err := w.Write(bytesCSV)
	if err != nil {
		logger.WithField("err", err).Errorln("csv export: failed to write csv")
	} else {
		logger.WithFields(logging.Fields{"bytesCSV": len(bytesCSV), "bytesWritten": bytesWritten}).Infoln("csv export: successfully wrote csv")
	}
}

const isoDateFormat = "2006-01-02"

func parseRange(fromStr, toStr string) (time.Time, time.Time, error) {
	from, err := time.Parse(isoDateFormat, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := time.Parse(isoDateFormat, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to = to.Add(24 * time.Hour) // Move "to" one day in the future, to include this entire date, regardless of the time, when doing time comparisons.
	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("%v is after %v", from, to)
	}
	return from, to, err
}

func isWithinRange(date, from, to time.Time) bool {
	return (date.After(from) || date.Equal(from)) && date.Before(to)
}

// Admin renders a website listing all organizations with their aggregations by month.
func (a *API) Admin(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	page := extractOrDefaultPage(r.URL.Query().Get("page"))

	resp, err := a.Users.GetOrganizations(r.Context(), &users.GetOrganizationsRequest{
		Query:          query,
		PageNumber:     int32(page),
		IncludeDeleted: true,
	})
	if err != nil {
		renderError(w, r, err)
		return
	}

	from, to := timeRange()
	ids, trialInfo := processOrgs(resp.Organizations, to)

	sums, err := a.DB.GetMonthSums(r.Context(), ids, from, to)
	if err != nil {
		renderError(w, r, err)
		return
	}

	instanceMonthSums, amountTypesMap := processSums(sums)
	user.LogWith(r.Context(), logging.Global()).Debugf("instanceMonthSums: %#v", instanceMonthSums)
	amountTypes, colors := processAmountTypes(amountTypesMap)
	months := months(from, to)

	now := time.Now().UTC()

	render.HTMLTemplate(w, http.StatusOK, a.adminTemplate, map[string]interface{}{
		"AdminURL":           a.AdminURL,
		"Organizations":      resp.Organizations,
		"TrialInfo":          trialInfo,
		"Months":             months,
		"AmountTypes":        amountTypes,
		"Colors":             colors,
		"sums":               instanceMonthSums,
		"Page":               page,
		"NextPage":           page + 1,
		"Query":              query,
		"UsageFrom":          sixMonthsAgo(now).Format(isoDateFormat),
		"UsageTo":            now.Format(isoDateFormat),
		"BillingFeatureFlag": featureflag.Billing,
	})
}

func extractOrDefaultPage(pageQueryArg string) int64 {
	page, _ := strconv.ParseInt(pageQueryArg, 10, 32)
	if page <= 0 {
		return 1
	}
	return page
}

func timeRange() (time.Time, time.Time) {
	now := time.Now().UTC()
	return sixMonthsAgo(now), now
}

func sixMonthsAgo(t time.Time) time.Time {
	// 6 months back from this month's start. We can't just do (now - 6 months),
	// as after the 28th, that will skip february, so we have to find the first
	// of this month, *then* calculate from there. *BUT*, we use now for the
	// end-time so that we include records for this month, which is incomplete.
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -6, 0)
}

func orgIDs(entries []*users.SummaryEntry) []string {
	var ids []string
	for _, entry := range entries {
		ids = append(ids, entry.OrgID)
	}
	return ids
}

func processOrgs(orgs []users.Organization, to time.Time) ([]string, map[string]trial.Trial) {
	var ids []string
	trialInfo := map[string]trial.Trial{}
	for _, org := range orgs {
		ids = append(ids, org.ID)
		trialInfo[org.ID] = trial.Info(org.TrialExpiresAt, org.CreatedAt, to)
	}
	return ids, trialInfo
}

func months(from, to time.Time) []time.Month {
	var months []time.Month
	for t := to; t.After(from) || t.Equal(from); t = t.AddDate(0, -1, 0) {
		months = append(months, t.Month())
	}
	return months
}

// From: https://stackoverflow.com/a/35482679/506244
func secondsInMonth(y int, m time.Month) int {
	start := time.Date(y, m, 0, 0, 0, 0, 0, time.UTC)
	end := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC)
	return int(end.Sub(start).Seconds())
}

func processSums(sums map[string][]db.Aggregate) (instanceMonthSums, map[string]struct{}) {
	instanceMonthSums := instanceMonthSums{}
	// Actually a set
	amountTypesMap := map[string]struct{}{}
	// Loop over instances
	for instanceID, aggs := range sums {
		monthSums := monthSums{}
		// Loop over month sums
		for _, agg := range aggs {
			s, ok := monthSums[agg.BucketStart.Month()]
			if !ok {
				s = totalSums{}
			}
			s[agg.AmountType] += agg.AmountValue
			monthSums[agg.BucketStart.Month()] = s

			amountTypesMap[agg.AmountType] = struct{}{}
		}
		// Loop over months to produce node-usage metric
		// Note, loop mutates monthSums
		// @TODO Specifying 2019 for year, terrible hack -- year should be preserved in data from db.
		for month, totalSums := range monthSums {
			totalSums["node-usage"] = int64(
				(float64(totalSums["node-seconds"]) / float64(secondsInMonth(2019, month))) * float64(nodeUsagePrecision))
		}
		amountTypesMap["node-usage"] = struct{}{}

		instanceMonthSums[instanceID] = monthSums
	}
	return instanceMonthSums, amountTypesMap
}

func processAmountTypes(amountTypesMap map[string]struct{}) ([]string, map[string]string) {
	var amountTypes []string
	colors := map[string]string{}
	for t := range amountTypesMap {
		amountTypes = append(amountTypes, t)
		colors[t] = amountTypeColor(t)
	}
	sort.Strings(amountTypes)
	return amountTypes, colors
}

// colors is taken from material 300-weight colours for a pleasing display.
var colors = []string{
	"#e57373", "#F06292", "#BA68C8", "#9575CD", "#7986CB", "#64B5F6", "#4FC3F7",
	"#4DD0E1", "#4DB6AC", "#81C784", "#AED581", "#DCE775", "#FFF176", "#FFD54F",
	"#FFB74D", "#FF8A65", "#A1887F", "#E0E0E0", "#90A4AE",
}

func amountTypeColor(amountType string) string {
	if len(amountType) == 0 {
		return colors[0]
	}
	return colors[int(amountType[0])%len(colors)]
}

var adminTemplate = `
{{ $admurl := .AdminURL }}
{{ $months := .Months }}
{{ $sums := .sums }}
{{ $colors := .Colors }}
{{ $trialInfo := .TrialInfo}}
{{ $billing := .BillingFeatureFlag }}
<html>
	<head>
		<title>Instance/Organization Billing</title>
		<link rel="stylesheet" href="https://fonts.googleapis.com/icon?family=Material+Icons">
		<link rel="stylesheet" href="https://code.getmdl.io/1.3.0/material.indigo-pink.min.css">
		<script defer src="https://code.getmdl.io/1.3.0/material.min.js"></script>
	</head>
	<body>
		<header class="mdl-layout__header mdl-color--grey-100 mdl-color-text--grey-600 is-casting-shadow">
		<div class="mdl-layout__header-row">
            <span class="mdl-layout-title">
				<div class="material-icons">monetization_on</div>
				Instance/Organization billing
            </span>
			<div class="mdl-layout-spacer"></div>

			<form action="" method="GET">
				<input type="hidden" name="page" value="{{.Page}}" />
				<label class="mdl-button mdl-js-button mdl-button--icon" for="query">
					<i class="material-icons">search</i>
				</label>
				<div class="mdl-textfield mdl-js-textfield">
					<input class="mdl-textfield__input" type="text" name="query" id="query" value="{{.Query}}">
					<label class="mdl-textfield__label" for="query">Search for InstanceID</label>
				</div>
			</form>

			<div class="mdl-layout-spacer"></div>

			<form id="export-as-csv" action="/admin/billing.csv" method="GET">
				<div class="mdl-textfield mdl-js-textfield" style="width:180px;">
					<input class="mdl-textfield__input" type="text" name="from" id="from" value="{{.UsageFrom}}" maxlength="10" size="10">
					<label class="mdl-textfield__label" for="from">Usage from (yyyy-MM-dd)</label>
				</div>
				<div class="mdl-textfield mdl-js-textfield" style="width:180px;">
					<input class="mdl-textfield__input" type="text" name="to" id="to" value="{{.UsageTo}}" maxlength="10" size="10">
					<label class="mdl-textfield__label" for="to">Usage to (yyyy-MM-dd)</label>
				</div>
				<button type="submit" form="export-as-csv" value="Submit" class="mdl-button mdl-button--icon">
					<i class="material-icons">file_download</i>
				</button>
			</form>
		</div>
		</header>

		<div id="orgs">
			{{if .Query}}
			<div class="mdl-grid">
				<p>Displaying results for «{{.Query}}» <a href="{{$admurl}}/billing/organizations">show all</a></p>
			</div>
			{{end}}

			<div class="mdl-grid">
				{{/* TODO: Potentially delete this code (why is it here?) #########
				{{range $key, $value := $colors}}
					<span style="color: {{ $value }}">{{$key}}</span>
				{{- end}}
				*/}}
				<table class="mdl-data-table mdl-js-data-table">
					<thead>
						{{/*  ##### MONTHS HEADER WRITER ########
						<tr>
							<th></th><th></th><th></th>
							{{- range $months -}}
							  {{$month := .}}
							  {{- range $colors -}}
									<th class="mdl-data-table__cell--non-numeric">{{$month}}</th>
								{{- end -}}
							{{- end -}}
						</tr>
						*/}}
						<tr>
							<th class="mdl-data-table__cell--non-numeric">ID</th>
							<th class="mdl-data-table__cell--non-numeric">Instance</th>
							<th class="mdl-data-table__cell--non-numeric">Name</th>
							<th class="mdl-data-table__cell--non-numeric">BillingEnabled</th>
							<th class="mdl-data-table__cell--non-numeric">TrialRemaining</th>
							{{/* #### DATA HEADER WRITER ####
							{{range $months -}}
								{{ range $amountType, $color := $colors -}}
									<th style="color: {{$color}}">{{$amountType}}</th>
								{{- end }}
							{{- end }}
							*/}}
						</tr>
					</thead>
					{{range .Organizations }}
						{{ $org := . }}
						<tr>
							<td class="mdl-data-table__cell--non-numeric">{{.ID}}</td>
							<td class="mdl-data-table__cell--non-numeric">
								<a href="{{ $admurl }}/users/organizations?query=instance%3A{{ $org.ExternalID }}">{{ $org.ExternalID }}</a>
							</td>
							<td class="mdl-data-table__cell--non-numeric">
								{{if .Deleted}}<strike>{{.Name}}</strike>{{else}}{{.Name}}{{end}}
							</td>
							<td class="mdl-data-table__cell--non-numeric">{{$org.HasFeatureFlag $billing}}</td>
							<td class="mdl-data-table__cell--non-numeric">{{with (index $trialInfo $org.ID)}}{{.Remaining}}/{{.Length}} days{{else}}err{{end}}</td>
						</tr>

						{{/* NEW CODE BY CRAIG */}}
						<tr>
							<td colspan=5>
								<table class="mdl-data-table mdl-js-data-table">
									<thead>
										<th>Month</th>
										{{ range $amountType, $color := $colors -}}
											<th style="color: {{$color}}">{{$amountType}}</th>
										{{- end }}

									</thead>

									{{ range $k, $month := $months -}}
										<tr>
											<td class="mdl-data-table__cell--non-numeric">{{$month}}</td>
										  {{- range $amountType, $color := $colors -}}
												<td>
													{{- with (index $sums $org.ID) -}}
														{{- with (index . $month) -}}
															{{- with (index . $amountType) -}}
																<div style="color: {{$color}}">
																	{{- if (eq $amountType "node-usage") -}}
																		{{- renderNodeUsage . -}}
																	{{- else -}}
																		{{- . -}}
																	{{- end -}}
																</div>
															{{- end -}}
														{{- end -}}
													{{- end -}}
												</td>
											{{- end -}}
										</tr>
									{{end}}
								</table>
							</td>
						</tr>


					{{ end }}
				</table>
				<div class="mdl-cell mdl-cell--12-col">
					<div class="mdl-grid">
						<div class="mdl-layout-spacer"></div>
						Displaying {{len .Organizations}} organizations on this page<br/>
					</div>
					<div class="mdl-grid">
						<div class="mdl-layout-spacer"></div>
						<a href="?query={{.Query}}&page={{.NextPage}}" class="mdl-button mdl-js-button mdl-button--raised">
						  Next page
						</a>
					</div>
				</div>
			</div>
		</div>
	</body>
</html>
`
