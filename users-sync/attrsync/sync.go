package attrsync

import (
	"context"
	"time"

	"github.com/weaveworks/common/logging"
	commonUser "github.com/weaveworks/common/user"

	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/filter"
)

type AttributeSyncer struct {
	log               logging.Interface
	recentUsersTicker *time.Ticker
	staleUsersTicker  *time.Ticker
	work              chan filter.User
	quit              chan struct{}
	db                db.DB
	billingClient     billing_grpc.BillingClient
}

// New creates a attributeSyncer service
func New(log logging.Interface, db db.DB, billingClient billing_grpc.BillingClient) *AttributeSyncer {
	return &AttributeSyncer{
		log:               log,
		db:                db,
		billingClient:     billingClient,
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
				c.work <- filter.All // TODO
			case <-c.staleUsersTicker.C:
				c.work <- filter.All // TODO
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

		for _, user := range users {
			ctxWithID := commonUser.InjectUserID(ctx, user.ID)
			if err := c.syncUser(ctxWithID, user); err != nil {
				c.log.
					WithField("error", err).
					WithField("user", user).
					Errorln("Error syncing user")
			}
		}

		if len(users) == 0 {
			break
		}
		page++
	}
	return nil
}

func (c *AttributeSyncer) syncUser(ctx context.Context, user *users.User) error {

	c.userOrgAttributes(ctx, user)
	return nil
}
