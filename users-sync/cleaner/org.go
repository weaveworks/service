package cleaner

import (
	"context"
	"github.com/weaveworks/common/logging"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
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
	quit    chan struct{}
	urls    []string
	db      db.DB
	log     logging.Interface
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
func New(urls []string, log logging.Interface, db db.DB) *OrgCleaner {
	return &OrgCleaner{
		log:     log,
		ticker:  time.NewTicker(tick).C,
		trigger: make(chan struct{}, 1),
		quit:    make(chan struct{}),
		urls:    urls,
		db:      db,
	}
}

// Start runs org cleaner
// runs findAndClean at the beginning and then on every tick and trigger
func (c *OrgCleaner) Start() {
	c.log.Infoln("Run org cleaner")
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		for {
			c.findAndClean(ctx)
			select {
			case <-c.ticker:
			case <-c.trigger:
			case <-c.quit:
				cancel()
				return
			}
		}
	}()
}

// Stop stops the OrgCleaner service
func (c *OrgCleaner) Stop() error {
	close(c.quit)
	return nil
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
		c.log.Warnf("cannot find uncleaned org IDs, error: %s\n", err)
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
			c.log.Warnf("cannot set cleanup for org id %s, error: %s\n", id, err)
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
			c.log.Warnf("failed to clean %s, error %s\n", url, err)
			errorCallEndpointTotal.With(prometheus.Labels{"url": url}).Inc()
			res = false
		}
		if !isDone(status) {
			res = false
		}
	}
	if !res {
		c.log.Infof("cleanup for org ID %s hasn't finished yet, will try again later\n", id)
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
