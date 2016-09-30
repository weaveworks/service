package marketing

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	pushPeriod = 1 * time.Minute
	batchSize  = 20
)

var (
	prospectsSent = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "marketing_prospects_sent",
			Help: "Marketing prospects sent.",
		},
		[]string{"service", "status"},
	)
)

func init() {
	prometheus.MustRegister(prospectsSent)
}

type prospect struct {
	Email             string    `json:"email"`
	ServiceCreatedAt  time.Time `json:"createdAt"`
	ServiceLastAccess time.Time `json:"lastAccess"`
}

func (p1 prospect) merge(p2 prospect) prospect {
	latest := func(t1, t2 time.Time) time.Time {
		if t1.After(t2) {
			return t1
		}
		return t2
	}

	email := p1.Email
	if email == "" {
		email = p2.Email
	}

	return prospect{
		Email:             email,
		ServiceCreatedAt:  latest(p1.ServiceCreatedAt, p2.ServiceCreatedAt),
		ServiceLastAccess: latest(p1.ServiceLastAccess, p2.ServiceLastAccess),
	}
}

// Queue for sending updates to marketing.
type Queue struct {
	sync.Mutex
	cond   *sync.Cond
	quit   chan struct{}
	client client

	// We don't send every 'hit', we
	// batch them up and dedupe them.
	hits map[string]time.Time

	// We also don't send prospect updates
	// synchronously - we queue them.
	prospects []prospect
}

type client interface {
	name() string
	batchUpsertProspect(prospects []prospect) error
}

// NewQueue makes a new marketing queue.
func NewQueue(client client) *Queue {
	queue := &Queue{
		client: client,
		quit:   make(chan struct{}),
		hits:   map[string]time.Time{},
	}
	queue.cond = sync.NewCond(&queue.Mutex)
	go queue.loop()
	go queue.periodicWakeUp()
	return queue
}

// Stop the pardot client.
func (c *Queue) Stop() {
	close(c.quit)
}

func (c *Queue) periodicWakeUp() {
	// Every period we wake up the condition
	// and have it push what ever hits we've
	// batched up.
	for ticker := time.Tick(pushPeriod); ; <-ticker {
		c.cond.Broadcast()
	}
}

func (c *Queue) loop() {
	for {
		c.waitForStuffToDo()
		c.push()

		select {
		case <-c.quit:
			return
		default:
		}
	}
}

func (c *Queue) waitForStuffToDo() {
	c.Lock()
	defer c.Unlock()
	for len(c.hits)+len(c.prospects) == 0 {
		c.cond.Wait()
	}
}

func (c *Queue) push() {
	accesses, creations := c.swap()

	prospectsByEmail := map[string]prospect{}
	for _, prospect := range creations {
		prospectsByEmail[prospect.Email] = prospect
	}
	for email, timestamp := range accesses {
		prospectsByEmail[email] = prospectsByEmail[email].merge(prospect{
			Email:             email,
			ServiceLastAccess: timestamp,
		})
	}

	prospects := []prospect{}
	for _, prospect := range prospectsByEmail {
		prospects = append(prospects, prospect)
	}

	name := c.client.name()
	log.Infof("Pushing %d prospect updates to %s", len(prospects), name)
	for i := 0; i < len(prospects); {
		end := i + batchSize
		if end > len(prospects) {
			end = len(prospects)
		}
		err := c.client.batchUpsertProspect(prospects[i:end])
		if err != nil {
			prospectsSent.WithLabelValues(name, "failed").Observe(float64(end - i))
			log.Errorf("Error pushing prospects: %v", err)
		} else {
			prospectsSent.WithLabelValues(name, "success").Observe(float64(end - i))
		}
		i = end
	}
}

func (c *Queue) swap() (map[string]time.Time, []prospect) {
	c.Lock()
	defer func() {
		c.hits = map[string]time.Time{}
		c.prospects = []prospect{}
		c.Unlock()
	}()
	return c.hits, c.prospects
}

// UserAccess should be called every time a users authenticates
// with the service.  These 'hits' will be batched up and only the
// latest sent periodically, so its okay to call this function very often.
func (c *Queue) UserAccess(email string, hitAt time.Time) {
	if c == nil {
		return
	}
	c.Lock()
	defer c.Unlock()
	c.hits[email] = hitAt
	// No broadcast here, we only do this periodically.
}

// UserCreated should be called when new users are created.
// This will trigger an immediate 'upload' to pardot, although
// that upload will still happen in the background.
func (c *Queue) UserCreated(email string, createdAt time.Time) {
	if c == nil {
		return
	}
	c.Lock()
	defer c.Unlock()
	c.prospects = append(c.prospects, prospect{
		Email:            email,
		ServiceCreatedAt: createdAt,
	})
	c.cond.Broadcast()
}

// Queues is a list of Queue; it handles the fanout.
type Queues []*Queue

// UserAccess calls UserAccess on each Queue.
func (qs Queues) UserAccess(email string, hitAt time.Time) {
	for _, q := range qs {
		q.UserAccess(email, hitAt)
	}
}

// UserCreated calls UserCreated on each Queue.
func (qs Queues) UserCreated(email string, createdAt time.Time) {
	for _, q := range qs {
		q.UserCreated(email, createdAt)
	}
}
