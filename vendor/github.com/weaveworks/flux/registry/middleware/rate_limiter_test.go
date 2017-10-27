package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const requestTimeout = 10 * time.Second

func TestRemoteFactory_RateLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	rateLimit := 100

	// Create a new rate limiter with a burst of 1 (i.e. single client, must wait)
	rl := RateLimitedRoundTripper(http.DefaultTransport, RateLimiterConfig{
		RPS:   rateLimit,
		Burst: 1,
	}, ts.URL)

	// Rate limited http client
	client := &http.Client{
		Transport: rl,
		Timeout:   requestTimeout,
	}

	// Number of non-erroring requests
	var count uint32
	// Time we started requesting
	start := time.Now()

	// Run this for 500 ms for a quick test, but enough samples to get a robust answer
	for time.Now().Before(start.Add(500 * time.Millisecond)) {
		go func() {
			_, err := client.Get(ts.URL)
			if err == nil {
				atomic.AddUint32(&count, 1)
			}
		}()
		time.Sleep(time.Millisecond) // Sleep for a millisecond to allow golang to do it's syscalls.
	}

	observedRateLimit := int(float64(atomic.LoadUint32(&count)) / (time.Now().Sub(start).Seconds()))
	if observedRateLimit < rateLimit-5 || observedRateLimit > rateLimit+5 {
		t.Fatalf("Expected rate limit of %v but got %v. We might need to widen the test.", rateLimit, observedRateLimit)
	}
}

// We shouldn't share roundtrippers in the ratelimiter because the context will be stale
func TestRateLimiter_WithContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	rateLimit := 100

	// A context we'll use to cancel
	ctx, cancel := context.WithCancel(context.Background())
	rt := &ContextRoundTripper{Transport: http.DefaultTransport, Ctx: ctx}
	rl := RateLimitedRoundTripper(rt, RateLimiterConfig{
		RPS:   rateLimit,
		Burst: 1,
	}, ts.URL)

	client := &http.Client{
		Transport: rl,
		Timeout:   requestTimeout,
	}
	_, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	cancel() // Perform the cancel. If the RL is sharing contexts, then if we use it again it will fail.

	// Now do that again, it should use a new context
	// A context we'll use to cancel requests on error
	ctx, cancel = context.WithCancel(context.Background())
	rt = &ContextRoundTripper{Transport: http.DefaultTransport, Ctx: ctx}
	rl = RateLimitedRoundTripper(rt, RateLimiterConfig{
		RPS:   rateLimit,
		Burst: 1,
	}, ts.URL)
	client = &http.Client{
		Transport: rl,
		Timeout:   requestTimeout,
	}
	_, err = client.Get(ts.URL)
	if err != nil {
		t.Fatal(err) // It will fail here if it is sharing contexts
	}
	cancel()
}
