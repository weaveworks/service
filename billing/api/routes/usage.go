package routes

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/weaveworks/service/billing/api/render"
	"github.com/weaveworks/service/users"
)

// Usage is non-extensible, but is kept as-is to prevent having to change
// the frontend js code.
type Usage struct {
	Start       string `json:"start"`
	NodeSeconds int64  `json:"nodeSeconds"`
}

// GetUsage returns an organization's usage. It supports form values
// `start` and `end` for time range.
func (a *API) GetUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	through := time.Now().UTC()
	from := through.Add(-24 * time.Hour)

	var err error
	if start := r.FormValue("start"); start != "" {
		from, err = parseTime(start)
		if err != nil {
			render.Error(w, r, err)
			return
		}
	}
	if end := r.FormValue("end"); end != "" {
		through, err = parseTime(end)
		if err != nil {
			render.Error(w, r, err)
			return
		}
	}

	org, err := a.Users.GetOrganization(ctx, &users.GetOrganizationRequest{
		ExternalID: mux.Vars(r)["id"],
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}

	aggs, err := a.DB.GetAggregates(ctx, org.Organization.ID, from, through)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	var usages []Usage
	for _, agg := range aggs {
		if agg.AmountType != "node-seconds" {
			continue
		}
		usages = append(usages, Usage{
			Start:       render.Time(agg.BucketStart),
			NodeSeconds: agg.AmountValue,
		})
	}
	render.JSON(w, http.StatusOK, usages)
}

func parseTime(in string) (t time.Time, err error) {
	return time.Parse(time.RFC3339Nano, in)
}
