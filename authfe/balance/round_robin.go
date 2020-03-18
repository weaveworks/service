// Wrappper for go-kit load-balancer so we can have a common interface
// between round-robin and bounded-load-consistent.
package balance

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/sd"
	"github.com/go-kit/kit/sd/dnssrv"
	"github.com/go-kit/kit/sd/lb"
	"github.com/weaveworks/common/logging"
)

// Balancer is an interface abstracting load balancers.
type Balancer interface {
	// Get returns the Service Endpoint to use for the next request, for an affinity key.
	Get(key string) (Endpoint, error)
	// Put releases the Endpoint when it has finished processing the request.
	Put(endpoint Endpoint)
	// Shut down any goroutines, dispose of any resources
	Close()
}

type closer interface {
	Close()
}

type RoundRobin struct {
	balancer   lb.Balancer
	endpointer closer
}

var _ Balancer = &RoundRobin{}

func NewSRVRoundRobin(hostAndPort string) Balancer {
	logger := gokitAdapter{i: logging.Global()}
	// Poll DNS for updates every 5 seconds
	instancer := dnssrv.NewInstancer(hostAndPort, 5*time.Second, logger)
	return NewRoundRobin(instancer)
}

func NewRoundRobin(instancer sd.Instancer) *RoundRobin {
	logger := gokitAdapter{i: logging.Global()}
	endpointer := sd.NewEndpointer(instancer, endpointFactory, logger)
	return &RoundRobin{
		balancer:   lb.NewRoundRobin(endpointer),
		endpointer: endpointer,
	}
}

// Indirect via the go-kit Endpoint abstraction that we don't really need:
// create a function that just returns the instance name as an interface{}
func endpointFactory(instance string) (endpoint.Endpoint, io.Closer, error) {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			return &addressEndpoint{address: instance}, nil
		},
		nil, nil
}

func (rr *RoundRobin) Get(key string) (Endpoint, error) {
	endpointFn, err := rr.balancer.Endpoint()
	if err != nil {
		return nil, err
	}
	endpointResponse, err := endpointFn(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	addressEndpoint, ok := endpointResponse.(*addressEndpoint)
	if !ok {
		return nil, fmt.Errorf("proxy: unexpected loadbalancer response: %#v", endpointResponse)
	}
	return addressEndpoint, nil
}

func (rr *RoundRobin) Put(Endpoint) {
	// no-op
}

func (rr *RoundRobin) Close() {
	rr.endpointer.Close()
}
