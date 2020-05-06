package attrsync

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/analytics-go"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/common"
	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/marketing"
)

var attrsComputeDurationCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: common.PrometheusNamespace,
	Subsystem: "users_sync",
	Name:      "user_sync_attrs_duration_seconds",
	Help:      "Time taken to recompute user attributes.",
})

const (
	minus30Days       = -1 * 30 * 24 * time.Hour
	recentUsersPeriod = 3 * time.Hour
	staleUsersPeriod  = 3 * 24 * time.Hour
)

func init() {
	attrsComputeDurationCollector.Register()
}

// AttributeSyncer sends metadata about users to external services
type AttributeSyncer struct {
	log               logging.Interface
	recentUsersTicker *time.Ticker
	staleUsersTicker  *time.Ticker
	work              chan filter.User
	quit              chan struct{}
	db                db.DB
	billingClient     billing_grpc.BillingClient
	marketoClient     marketing.MarketoClient
}

// New creates a attributeSyncer service
func New(log logging.Interface, db db.DB, billingClient billing_grpc.BillingClient, marketoClient marketing.MarketoClient) *AttributeSyncer {
	return &AttributeSyncer{
		log:               log,
		db:                db,
		billingClient:     billingClient,
		marketoClient:     marketoClient,
		recentUsersTicker: time.NewTicker(recentUsersPeriod),
		staleUsersTicker:  time.NewTicker(staleUsersPeriod),
		work:              make(chan filter.User),
		quit:              make(chan struct{}),
	}
}

// Start starts the attributeSyncer service
func (c *AttributeSyncer) Start() error {
	go c.sync()
	go func() {
		for {

			select {
			case <-c.recentUsersTicker.C:
				// FIXME: Logged in since date is a poor proxy for recently 'active' users
				c.work <- filter.LoggedInSince(time.Now().Add(minus30Days))
			case <-c.staleUsersTicker.C:
				// FIXME: Logged in since date is a poor proxy for recently 'active' users
				c.work <- filter.NotLoggedInSince(time.Now().Add(minus30Days))
			case <-c.quit:
				close(c.work)
				return
			}
		}
	}()
	return nil
}

// Stop stops the attributeSyncer service
func (c *AttributeSyncer) Stop() error {
	c.recentUsersTicker.Stop()
	c.staleUsersTicker.Stop()
	close(c.quit)
	return nil
}

// EnqueueUsersSync enqueues a task to sync attributes for a given set of users
func (c *AttributeSyncer) EnqueueUsersSync(ctx context.Context, userIDs []string) error {
	c.work <- filter.UsersByID(userIDs)
	return nil
}

// EnqueueOrgsSync enqueues a task to sync attributes for a given set of orgs
func (c *AttributeSyncer) EnqueueOrgsSync(ctx context.Context, orgExternalIDs []string) error {
	// we could do this DB lookup later and remove it from the Enqueue call path
	// but it should be relatively cheap.
	for _, externalID := range orgExternalIDs {
		// We request memberships for deleted orgs too so that we update users who
		// were connected to the deleted orgs
		users, err := c.db.ListOrganizationUsers(ctx, externalID, true, false)
		if err != nil {
			return err
		}
		userIDs := make([]string, len(users))
		for _, user := range users {
			userIDs = append(userIDs, user.ID)
		}
		c.work <- filter.UsersByID(userIDs)
	}
	return nil
}

func (c *AttributeSyncer) sync() {
	ctx, cancel := context.WithCancel(context.Background())
	for {
		userFilter := <-c.work
		if userFilter == nil {
			cancel()
			return
		}
		span, syncCtx := opentracing.StartSpanFromContext(ctx, "Sync")
		span.LogKV("userFilter", userFilter)

		err := c.syncUsers(syncCtx, userFilter)
		if err != nil {
			c.log.WithField("error", err).Errorln("Error syncing users")
		}
	}
}

func (c *AttributeSyncer) syncUsers(ctx context.Context, userFilter filter.User) error {
	// page starts at 1 because 0 is "all pages"
	page := uint64(1)
	for {
		us, err := c.db.ListUsers(ctx, userFilter, page)
		if err != nil {
			return err
		}

		if len(us) == 0 {
			break
		}

		span, pageCtx := opentracing.StartSpanFromContext(ctx, "SyncUsersPage")
		span.LogKV("count", len(us), "page", page)
		c.log.WithFields(map[string]interface{}{
			"filter": userFilter,
			"count":  len(us),
			"page":   page,
		}).Debugf("Syncing attributes for users")

		c.postUsers(pageCtx, us)

		page++
	}
	return nil
}

func segmentTrait(user *users.User, attrs map[string]int) analytics.Traits {
	trait := analytics.NewTraits().SetEmail(user.Email).SetCreatedAt(user.CreatedAt)

	// Since old users won't have this data, send it optionally
	if user.Name != "" {
		trait.SetName(user.Name)
	}
	if user.FirstName != "" {
		trait.SetFirstName(user.FirstName)
	}
	if user.LastName != "" {
		trait.SetLastName(user.LastName)
	}
	if user.Company != "" {
		trait.Set("company", map[string]string{"name": user.Company})
	}

	for name, val := range attrs {
		trait.Set(name, val)
	}

	return trait
}

func (c *AttributeSyncer) postUsers(ctx context.Context, users []*users.User) {
	traits := map[string]analytics.Traits{}
	var prospects []marketing.Prospect
	for _, user := range users {
		if p, ok := marketoProspect(user); ok {
			prospects = append(prospects, p)
		}

		attrs, err := c.userOrgAttributes(ctx, user)
		if err != nil {
			c.log.WithField("err", err).WithField("user", user.ID).Errorln("Error getting org attributes for user")
			continue
		}
		traits[user.Email] = segmentTrait(user, attrs)
	}

	instrument.CollectedRequest(
		ctx,
		"syncUsers",
		attrsComputeDurationCollector,
		instrument.ErrorCode,
		func(fCtx context.Context) error {
			// Marketo
			if len(prospects) > 0 {
				if err := c.marketoClient.BatchUpsertProspect(prospects); err != nil {
					c.log.WithField("err", err).Errorln("Error sending fields to Marketo")
				}
			}

			return nil
		})
}
