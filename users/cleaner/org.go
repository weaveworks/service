package cleaner

import (
	"context"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	commonUser "github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users/db"
)

const (
	tick       = 60 * time.Second
	reqTimeout = 5 * time.Second
)

var (
	findAndCleanTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "cleaner",
		Name:      "find_and_clean_total",
		Help:      "Total number of findAndClean() iterations (independent of success / failure)."})

	errorFindUncleanedOrgIDsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "cleaner",
		Name:      "find_uncleaned_orgs_errors_total",
		Help:      "Total number of errors finding uncleaned organizations."})

	callEndpointTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "cleaner",
		Name:      "call_cleanup_endpoint_total",
		Help:      "Total number of cleanup requests per URL.",
	}, []string{"url"})

	errorCallEndpointTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "cleaner",
		Name:      "call_cleanup_endpoint_errors_total",
		Help:      "Total number of errors calling cleanup endpoints per URL.",
	}, []string{"url"})

	uncleanedOrgs = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "cleaner",
		Name:      "uncleaned_orgs",
		Help:      "Total number of currently uncleaned organizations."})

	cleanedOrgsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "cleaner",
		Name:      "cleaned_orgs_total",
		Help:      "Total number of fully cleaned organizations."})
)

// A OrgCleaner calls endpoints from urls list
// to clean up after instance deletion in database db.
// Cleaner checks for uncleaned orgs IDs by ticker and also might be triggered by trigger
type OrgCleaner struct {
	ticker  <-chan time.Time
	trigger chan struct{}
	urls    []string
	db      db.DB
}

func init() {
	prometheus.MustRegister(
		findAndCleanTotal,
		errorFindUncleanedOrgIDsTotal,
		callEndpointTotal,
		errorCallEndpointTotal,
		uncleanedOrgs,
		cleanedOrgsTotal,
	)
}

// New returns a new OrgCleaner given a list of URLs and database db.
func New(urls []string, db db.DB) *OrgCleaner {
	return &OrgCleaner{
		ticker:  time.NewTicker(tick).C,
		trigger: make(chan struct{}, 1),
		urls:    urls,
		db:      db,
	}
}

// Run runs org cleaner with context
// runs findAndClean at the beginning and then on every tick and trigger
func (c *OrgCleaner) Run(ctx context.Context) {
	log.Info("Run org cleaner")
	go func() {
		for {
			c.findAndClean(ctx)
			select {
			case <-c.ticker:
			case <-c.trigger:
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Trigger triggers cleaning by a non-blocking send to a trigger channel
// if blocked, will skip this one and clean on the next tick
func (c *OrgCleaner) Trigger() {
	select {
	case c.trigger <- struct{}{}:
	default:
	}
}

func (c *OrgCleaner) findAndClean(ctx context.Context) {
	findAndCleanTotal.Inc()
	ids, err := c.db.FindUncleanedOrgIDs(ctx)
	if err != nil {
		log.Warnf("cannot find uncleaned org IDs, error: %s", err)
		errorFindUncleanedOrgIDsTotal.Inc()
		return
	}
	uncleanedOrgs.Set(float64(len(ids)))
	for _, id := range ids {
		c.clean(ctx, id)
	}
}

func (c *OrgCleaner) clean(ctx context.Context, id string) {
	done := c.runCleanupJobs(ctx, id)
	if done {
		if err := c.db.SetOrganizationCleanup(ctx, id, true); err != nil {
			log.Warnf("cannot set cleanup for org id %s, error: %s", id, err)
			return
		}
		cleanedOrgsTotal.Inc()
	}
}

// runCleanupJobs tries to run all cleaning jobs for organization with id
// it returns true only if all jobs are successfully done
// if no jobs found, return false to cleanup later, when there are some jobs
func (c *OrgCleaner) runCleanupJobs(ctx context.Context, id string) bool {
	if len(c.urls) == 0 {
		return false
	}
	res := true
	for _, url := range c.urls {
		callEndpointTotal.With(prometheus.Labels{"url": url}).Inc()
		status, err := callEndpoint(ctx, id, url)
		if err != nil {
			log.Warnf("failed to clean %s, error %s", url, err)
			errorCallEndpointTotal.With(prometheus.Labels{"url": url}).Inc()
			res = false
		}
		if !isDone(status) {
			res = false
		}
	}
	if !res {
		log.Infof("cleanup for org ID %s hasn't finished yet, will try again later", id)
	}
	return res
}

func callEndpoint(ctx context.Context, id, url string) (int, error) {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrapf(err, "creating DELETE request to URL %s", url)
	}

	ctxWithID := commonUser.InjectOrgID(ctx, id)
	err = commonUser.InjectOrgIDIntoHTTPRequest(ctxWithID, req)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "cannot inject instance ID into request")
	}

	ctx, cancel := context.WithTimeout(ctx, reqTimeout)
	req = req.WithContext(ctx)
	defer cancel()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrapf(err, "executing DELETE request to URL %s", url)
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func isDone(s int) bool {
	return s == http.StatusOK || s == http.StatusNotFound
}
