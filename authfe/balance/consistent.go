package balance

import (
	"errors"
	"sync"
	"time"

	"github.com/go-kit/kit/sd"
	"github.com/go-kit/kit/sd/dnssrv"
	"github.com/weaveworks/common/logging"
)

// map dlespiau/balance into go-kit world

// ErrNoEndpoints means no endpoints were available to balance across
var ErrNoEndpoints = errors.New("no endpoints")

type consistentWrapper struct {
	mtx       sync.RWMutex
	instancer sd.Instancer
	cache     map[string]Endpoint
	ch        chan sd.Event
	c         *Consistent
}

var _ Balancer = &consistentWrapper{}

// NewSRVConsistent creates a load-balancer given a DNS SRV name like
// _http._tcp.collectionh.scope.svc.cluster.local.
func NewSRVConsistent(name, hostAndPort string, loadFactor float64) Balancer {
	logger := gokitAdapter{i: logging.Global()}
	// Poll DNS for updates every 5 seconds
	instancer := dnssrv.NewInstancer(hostAndPort, 5*time.Second, logger)
	return NewConsistentWrapper(name, instancer, loadFactor)
}

// NewConsistentWrapper creates a load-balancer given an instancer; mostly for testing.
func NewConsistentWrapper(name string, instancer sd.Instancer, loadFactor float64) Balancer {
	cw := &consistentWrapper{
		instancer: instancer,
		cache:     make(map[string]Endpoint),
		ch:        make(chan sd.Event),
		c: NewConsistent(ConsistentConfig{
			Name:       name,
			LoadFactor: loadFactor,
		}),
	}
	go cw.receive()
	instancer.Register(cw.ch)
	return cw
}

func (c *consistentWrapper) receive() {
	for event := range c.ch {
		c.update(event)
	}
}

func (c *consistentWrapper) update(event sd.Event) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if event.Err != nil {
		logging.Global().Errorf("error from sd: %w", event.Err)
		return
	}

	removals := []Endpoint{}
cacheLoop:
	for key, endpoint := range c.cache {
		// dumb O(n^2) algorithm should be fine for modest numbers of endpoints
		for _, instance := range event.Instances {
			if key == instance {
				continue cacheLoop
			}
		}
		logging.Global().Debugf("removing instance %s", key)
		removals = append(removals, endpoint)
	}
	c.c.RemoveEndpoints(removals...)

	additions := []Endpoint{}
	for _, instance := range event.Instances {
		if _, found := c.cache[instance]; !found {
			logging.Global().Debugf("adding instance %s", instance)
			// new one
			endpoint := &addressEndpoint{address: instance}
			c.cache[instance] = endpoint
			additions = append(additions, endpoint)
		}
	}
	c.c.AddEndpoints(additions...)
}

func (c *consistentWrapper) Get(key string) (Endpoint, error) {
	e := c.c.Get(key)
	if e == nil {
		return nil, ErrNoEndpoints
	}
	return e, nil
}

func (c *consistentWrapper) Put(endpoint Endpoint) {
	c.c.Put(endpoint)
}

func (c *consistentWrapper) Close() {
	c.instancer.Deregister(c.ch)
	close(c.ch)
}
