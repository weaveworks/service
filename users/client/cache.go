package client

import (
	"net/http"
	"time"

	"github.com/bluele/gcache"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/users"
)

var (
	authCacheCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: common.PrometheusNamespace,
		Name:      "auth_cache",
		Help:      "Reports fetches that miss local cache.",
	}, []string{"cache", "result"})
)

func init() {
	prometheus.MustRegister(authCacheCounter)
}

// CachingClientConfig control behaviour of the authenticator client.
type CachingClientConfig struct {
	CredCacheEnabled         bool
	ProbeCredCacheSize       int
	OrgCredCacheSize         int
	ProbeCredCacheExpiration time.Duration
	OrgCredCacheExpiration   time.Duration
}

type cachingClient struct {
	client         users.UsersClient
	probeCredCache gcache.Cache
	orgCredCache   gcache.Cache
}

func newCachingClient(cfg CachingClientConfig, client users.UsersClient) *cachingClient {
	return &cachingClient{
		client:         client,
		probeCredCache: gcache.New(cfg.ProbeCredCacheSize).LRU().Expiration(cfg.ProbeCredCacheExpiration).Build(),
		orgCredCache:   gcache.New(cfg.OrgCredCacheSize).LRU().Expiration(cfg.OrgCredCacheExpiration).Build(),
	}
}

type cacheValue struct {
	out interface{}
	err error
}

// LookupOrg authenticates a cookie for access to an org by extenal ID.
func (c *cachingClient) LookupOrg(ctx context.Context, in *users.LookupOrgRequest, opts ...grpc.CallOption) (*users.LookupOrgResponse, error) {
	if c.orgCredCache == nil {
		return c.client.LookupOrg(ctx, in, opts...)
	}

	org, err := c.orgCredCache.Get(*in)
	authCacheCounter.WithLabelValues("org_cred_cache", hitOrMiss(err)).Inc()
	if err == nil {
		return org.(cacheValue).out.(*users.LookupOrgResponse), org.(cacheValue).err
	}

	out, err := c.client.LookupOrg(ctx, in, opts...)
	if err == nil || isUnauthorized(err) {
		c.orgCredCache.Set(*in, cacheValue{out, err})
	}
	return out, err
}

// LookupUsingToken authenticates a token for access to an org.
func (c *cachingClient) LookupUsingToken(ctx context.Context, in *users.LookupUsingTokenRequest, opts ...grpc.CallOption) (*users.LookupUsingTokenResponse, error) {
	if c.probeCredCache == nil {
		return c.client.LookupUsingToken(ctx, in, opts...)
	}

	org, err := c.probeCredCache.Get(*in)
	authCacheCounter.WithLabelValues("probe_cred_cache", hitOrMiss(err)).Inc()
	if err == nil {
		return org.(cacheValue).out.(*users.LookupUsingTokenResponse), org.(cacheValue).err
	}

	out, err := c.client.LookupUsingToken(ctx, in, opts...)
	if err == nil || isUnauthorized(err) {
		c.probeCredCache.Set(*in, cacheValue{out, err})
	}
	return out, err
}

// LookupAdmin authenticates a cookie for admin access.
func (c *cachingClient) LookupAdmin(ctx context.Context, in *users.LookupAdminRequest, opts ...grpc.CallOption) (*users.LookupAdminResponse, error) {
	return c.client.LookupAdmin(ctx, in, opts...)
}

// LookupUser authenticates a cookie.
func (c *cachingClient) LookupUser(ctx context.Context, in *users.LookupUserRequest, opts ...grpc.CallOption) (*users.LookupUserResponse, error) {
	return c.client.LookupUser(ctx, in, opts...)
}

func hitOrMiss(err error) string {
	if err == nil {
		return "hit"
	}
	return "miss"
}

func isUnauthorized(err error) bool {
	unauthorized, ok := err.(*Unauthorized)
	if !ok {
		return false
	}
	return unauthorized.httpStatus == http.StatusUnauthorized
}
