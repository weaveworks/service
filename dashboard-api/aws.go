package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/weaveworks/service/dashboard-api/aws"

	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common/render"
)

// GetAWSResources returns the list of AWS-managed services from CloudWatch-exported metrics.
func (api *API) GetAWSResources(w http.ResponseWriter, r *http.Request) {
	orgID, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	ctx, cancel := context.WithTimeout(ctx, api.cfg.prometheus.timeout)
	defer cancel()

	log.WithField("orgID", orgID).WithField("url", r.URL).Debug("GetAWSResources")

	resources, err := api.getAWSResources(ctx)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, resources)
}

// AWS resources,
// - grouped by type,
// - in the order we want these to be rendered,
// - with names sorted alphabetically.
type resources struct {
	Type     aws.Type     `json:"type"` // e.g. ELB, RDS, SQS, etc.
	Category aws.Category `json:"category"`
	Names    []string     `json:"names"`
}

func (api *API) getAWSResources(ctx context.Context) ([]resources, error) {
	to := time.Now()
	from := to.Add(-1 * time.Hour) // Not too long in the past so that Cortex serves the series from memcached.
	labelSets, err := api.prometheus.Series(ctx, []string{fmt.Sprintf("{kubernetes_namespace=\"%v\",_weave_service=\"%v\"}", aws.Namespace, aws.Service)}, from, to)
	if err != nil {
		return nil, err
	}
	return labelSetsToResources(labelSets), nil
}

func labelSetsToResources(labelSets []model.LabelSet) []resources {
	resourcesSets := make(map[aws.Product]map[string]bool)
	for _, labelSet := range labelSets {
		for _, product := range aws.Products {
			if resourceName, ok := labelSet[product.LabelName]; ok {
				if _, ok := resourcesSets[product]; !ok {
					resourcesSets[product] = make(map[string]bool)
				}
				resourcesSets[product][string(resourceName)] = true
				break
			}
		}
	}

	resourcesArray := []resources{}
	for _, product := range aws.Products {
		if set, ok := resourcesSets[product]; ok {
			resourcesArray = append(resourcesArray, resources{
				Type:     product.Type,
				Category: product.Category,
				Names:    setToSortedArray(set),
			})
		}
	}

	return resourcesArray
}

func setToSortedArray(set map[string]bool) []string {
	array := setToArray(set)
	sort.Strings(array)
	return array
}

func setToArray(set map[string]bool) []string {
	array := make([]string, len(set))
	i := 0
	for elem := range set {
		array[i] = elem
		i++
	}
	return array
}
