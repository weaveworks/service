package attrsync

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/analytics-go"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	commonUser "github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common"
	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/filter"
)

var attrsComputeDurationCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: common.PrometheusNamespace,
	Subsystem: "users_sync",
	Name:      "user_sync_attrs_duration_seconds",
	Help:      "Time taken to recompute user attributes.",
})

const minus30Days = -1 * 30 * 24 * time.Hour

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
	segmentClient     analytics.Client
}

// New creates a attributeSyncer service
func New(log logging.Interface, db db.DB, billingClient billing_grpc.BillingClient, segmentClient analytics.Client) *AttributeSyncer {
	return &AttributeSyncer{
		log:               log,
		db:                db,
		billingClient:     billingClient,
		segmentClient:     segmentClient,
		recentUsersTicker: time.NewTicker(3 * time.Hour),
		staleUsersTicker:  time.NewTicker(3 * 24 * time.Hour),
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
		users, err := c.db.ListOrganizationUsers(ctx, externalID)
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

		err := c.syncUsers(ctx, userFilter)
		c.log.WithField("error", err).Errorln("Error syncing users")
	}
}

func (c *AttributeSyncer) syncUsers(ctx context.Context, userFilter filter.User) error {
	page := uint64(0)
	for {
		users, err := c.db.ListUsers(ctx, userFilter, page)
		if err != nil {
			return err
		}
		c.log.WithFields(map[string]interface{}{
			"filter": userFilter,
			"count":  len(users),
			"page":   page,
		}).Debugf("Syncing attributes for users")

		if len(users) == 0 {
			break
		}

		for _, user := range users {
			ctxWithID := commonUser.InjectUserID(ctx, user.ID)
			instrument.CollectedRequest(
				ctxWithID,
				"syncUser",
				attrsComputeDurationCollector,
				instrument.ErrorCode,
				func(fCtx context.Context) error {
					err := c.syncUser(fCtx, user)
					if err != nil {
						c.log.WithField("error", err).
							WithField("user", user).
							Errorln("Error syncing user")
					}
					return err
				})

			// Sleep for a while to avoid overloading other services
			time.Sleep(10 * time.Millisecond)
		}

		page++
	}
	return nil
}

func (c *AttributeSyncer) syncUser(ctx context.Context, user *users.User) error {
	attrs, err := c.userOrgAttributes(ctx, user)

	if err != nil {
		return err
	}

	traits := analytics.NewTraits().
		SetName(user.Name).
		SetEmail(user.Email).
		SetCreatedAt(user.CreatedAt).
		Set("company", map[string]string{"name": user.Company})

	for name, val := range attrs {
		traits.Set(name, val)
	}

	if c.segmentClient == nil {
		c.log.WithField("traits", traits).Warnf("No segment client, skipping sending traits")
	} else {
		c.segmentClient.Enqueue(analytics.Identify{
			UserId: user.Email,
			Traits: traits,
		})
	}
	return nil
}
