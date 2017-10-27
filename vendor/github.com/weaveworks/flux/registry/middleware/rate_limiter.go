package middleware

import (
	"context"
	"net/http"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

var limiters = make(map[string]*rate.Limiter)
var limitersMutex sync.Mutex

type RateLimiterConfig struct {
	RPS   int // Rate per second per host
	Burst int // Burst count per host
}

func RateLimitedRoundTripper(rt http.RoundTripper, config RateLimiterConfig, host string) http.RoundTripper {
	limitersMutex.Lock()
	if _, ok := limiters[host]; !ok {
		rl := rate.NewLimiter(rate.Limit(config.RPS), config.Burst)
		limiters[host] = rl
	}
	limitersMutex.Unlock()
	return &RoundTripRateLimiter{
		RL:        limiters[host],
		Transport: rt,
	}
}

type RoundTripRateLimiter struct {
	RL        *rate.Limiter
	Transport http.RoundTripper
}

func (rl *RoundTripRateLimiter) RoundTrip(r *http.Request) (*http.Response, error) {
	// Wait errors out if the request cannot be processed within
	// the deadline. This is preemptive, instead of waiting the
	// entire duration.
	if err := rl.RL.Wait(r.Context()); err != nil {
		return nil, errors.Wrap(err, "rate limited")
	}
	return rl.Transport.RoundTrip(r)
}

type ContextRoundTripper struct {
	Transport http.RoundTripper
	Ctx       context.Context
}

func (rt *ContextRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.Transport.RoundTrip(r.WithContext(rt.Ctx))
}
